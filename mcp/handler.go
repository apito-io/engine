package mcp

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

const (
	mcpProtocol = "2025-06-18"
	accessTTL   = time.Hour
	refreshTTL  = 30 * 24 * time.Hour
	codeTTL     = time.Minute
	requestTTL  = 10 * time.Minute
)

// SystemAPI executes an already-authorized system call inside the engine process.
type SystemAPI interface {
	GraphQL(ctx context.Context, token, projectID, tenantID, query string, vars map[string]any) (json.RawMessage, error)
	REST(ctx context.Context, token, method, path string, body []byte) (int, json.RawMessage, error)
}

// Auditor records MCP tool calls on the existing log pipeline.
type Auditor interface {
	Record(ctx context.Context, event AuditEvent)
}

// AuditEvent is trusted server-side MCP metadata.
type AuditEvent struct {
	RequestID string `json:"request_id"`
	Tool      string `json:"tool"`
	Transport string `json:"transport"`
	Edition   string `json:"edition"`
	ProjectID string `json:"project_id,omitempty"`
	TenantID  string `json:"tenant_id,omitempty"`
	ScopeOK   bool   `json:"scope_ok"`
	Error     string `json:"error,omitempty"`
}

// Options wires the adapter to engine auth without a second permission model.
type Options struct {
	Config    *models.Config
	StorePath string
	Authorize echo.MiddlewareFunc
	Mint      func(ctx context.Context, userID string, req *models.CreateAccessTokenRequest) (string, error)
	System    SystemAPI
	Audit     Auditor
	Edition   string
	Now       func() time.Time
}

// Handler serves OAuth discovery and Streamable HTTP MCP on the engine router.
type Handler struct {
	opt      Options
	store    *Store
	leases   sync.Map
	sessions sync.Map
	limits   sync.Map
}

type lease struct {
	ProjectID string
	TenantID  string
	Expires   time.Time
}

func Mount(e *echo.Echo, opt Options) (*Handler, error) {
	path := opt.StorePath
	if path == "" && opt.Config != nil {
		path = strings.TrimSpace(opt.Config.MCPOAuthStore)
	}
	if path == "" {
		base := ""
		if opt.Config != nil {
			base = strings.TrimSpace(opt.Config.SQLiteDataDir)
			if base == "" {
				base = strings.TrimSpace(opt.Config.DefaultDatabaseDir)
			}
		}
		if base == "" {
			base = "."
		}
		path = base + "/mcp-oauth.json"
	}
	store, err := OpenStore(path)
	if err != nil {
		return nil, err
	}
	if opt.Edition == "" {
		opt.Edition = "pro"
	}
	if opt.Now == nil {
		opt.Now = time.Now
	}
	h := &Handler{opt: opt, store: store}
	e.GET("/.well-known/oauth-protected-resource", h.protectedResource)
	e.GET("/.well-known/oauth-protected-resource/mcp", h.protectedResource)
	e.GET("/.well-known/oauth-authorization-server", h.authorizationServer)
	e.POST("/oauth/register", h.register)
	e.GET("/oauth/authorize", h.authorize)
	e.POST("/oauth/token", h.token)
	e.GET("/mcp", h.mcp)
	e.POST("/mcp", h.mcp)
	e.DELETE("/mcp", h.mcp)
	if opt.Authorize != nil {
		approved := e.Group("/system/oauth/mcp", opt.Authorize)
		approved.POST("/approve", h.approve)
	}
	return h, nil
}

func (h *Handler) now() time.Time {
	if h.opt.Now != nil {
		return h.opt.Now()
	}
	return time.Now()
}

func (h *Handler) resource(c echo.Context) string {
	configured := ""
	if h.opt.Config != nil {
		configured = h.opt.Config.MCPPublicURL
	}
	base := PublicBase(c.Scheme(), c.Request().Host, c.Request().Header.Get("X-Forwarded-Proto"), c.Request().Header.Get("X-Forwarded-Host"), configured)
	if base == "" {
		return LocalMCPURL
	}
	if strings.HasSuffix(base, "/mcp") {
		resource, err := CanonicalResource(base)
		if err == nil {
			return resource
		}
	}
	return strings.TrimRight(base, "/") + "/mcp"
}

func (h *Handler) issuer(c echo.Context) string {
	resource := h.resource(c)
	parsed, err := url.Parse(resource)
	if err != nil {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

func (h *Handler) protectedResource(c echo.Context) error {
	resource := h.resource(c)
	return c.JSON(http.StatusOK, map[string]any{
		"resource":                 resource,
		"authorization_servers":    []string{h.issuer(c)},
		"bearer_methods_supported": []string{"header"},
		"scopes_supported":         []string{ScopeMCP},
	})
}

func (h *Handler) authorizationServer(c echo.Context) error {
	issuer := h.issuer(c)
	return c.JSON(http.StatusOK, map[string]any{
		"issuer":                                issuer,
		"authorization_endpoint":                issuer + "/oauth/authorize",
		"token_endpoint":                        issuer + "/oauth/token",
		"registration_endpoint":                 issuer + "/oauth/register",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code", "refresh_token", "urn:ietf:params:oauth:grant-type:token-exchange"},
		"code_challenge_methods_supported":      []string{"S256"},
		"token_endpoint_auth_methods_supported": []string{"none"},
		"scopes_supported":                      []string{ScopeMCP},
	})
}

func (h *Handler) register(c echo.Context) error {
	if !h.allow(c, 30) {
		return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "slow_down"})
	}
	var body struct {
		RedirectURIs []string `json:"redirect_uris"`
		ClientName   string   `json:"client_name"`
		AuthMethod   string   `json:"token_endpoint_auth_method"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_client_metadata"})
	}
	if body.AuthMethod != "" && body.AuthMethod != "none" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_client_metadata"})
	}
	if len(body.RedirectURIs) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_redirect_uri"})
	}
	for _, raw := range body.RedirectURIs {
		if err := validateRedirect(raw); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_redirect_uri"})
		}
	}
	id := "mcp_" + randomToken(16)
	rec := ClientRecord{ID: id, Name: strings.TrimSpace(body.ClientName), RedirectURIs: body.RedirectURIs, CreatedAt: h.now()}
	if err := h.store.SaveClient(rec); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "server_error"})
	}
	return c.JSON(http.StatusCreated, map[string]any{
		"client_id":                  id,
		"client_name":                rec.Name,
		"redirect_uris":              rec.RedirectURIs,
		"token_endpoint_auth_method": "none",
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
	})
}

func (h *Handler) authorize(c echo.Context) error {
	if c.QueryParam("response_type") != "code" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported_response_type"})
	}
	if c.QueryParam("code_challenge_method") != "S256" || len(c.QueryParam("code_challenge")) < 43 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request", "error_description": "S256 PKCE is required"})
	}
	client, ok := h.store.Client(c.QueryParam("client_id"))
	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_client"})
	}
	redirectURI := c.QueryParam("redirect_uri")
	if !exactRedirect(client.RedirectURIs, redirectURI) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_redirect_uri"})
	}
	resource, err := CanonicalResource(c.QueryParam("resource"))
	if err != nil || resource != h.resource(c) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_target", "error_description": "resource does not match this MCP endpoint"})
	}
	scope := c.QueryParam("scope")
	if scope == "" {
		scope = ScopeMCP
	}
	if scope != ScopeMCP {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_scope"})
	}
	id := randomToken(24)
	rec := AuthRequest{
		ID: id, ClientID: client.ID, RedirectURI: redirectURI,
		CodeChallenge: c.QueryParam("code_challenge"), Method: "S256",
		Resource: resource, State: c.QueryParam("state"), Scope: scope,
		ExpiresAt: h.now().Add(requestTTL), Status: "pending",
	}
	if err := h.store.SaveRequest(rec); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "server_error"})
	}
	origin := strings.TrimRight(h.issuer(c), "/")
	if h.opt.Config != nil && strings.TrimSpace(h.opt.Config.CORSOrigin) != "" {
		origin = strings.TrimRight(strings.TrimSpace(h.opt.Config.CORSOrigin), "/")
	}
	q := url.Values{}
	q.Set("request_id", id)
	q.Set("resource", resource)
	return c.Redirect(http.StatusFound, origin+"/mcp?"+q.Encode())
}

type approvalBody struct {
	RequestID         string              `json:"request_id"`
	ProjectGrantMode  string              `json:"project_grant_mode"`
	ProjectIDs        []string            `json:"project_ids"`
	TenantGrantMode   string              `json:"tenant_grant_mode"`
	TenantIDs         map[string][]string `json:"tenant_ids"`
	AcknowledgeDanger bool                `json:"acknowledge_danger"`
}

func (h *Handler) approve(c echo.Context) error {
	userID, _ := c.Get("user").(string)
	if strings.TrimSpace(userID) == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": "sign in to the console before approving MCP"})
	}
	var body approvalBody
	if err := c.Bind(&body); err != nil || strings.TrimSpace(body.RequestID) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "request_id is required"})
	}
	rec, ok := h.store.Request(body.RequestID)
	if !ok || h.now().After(rec.ExpiresAt) || rec.Status != "pending" {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "authorization request expired"})
	}
	if h.opt.Mint == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "access token service unavailable"})
	}
	label := rec.ID
	if len(label) > 8 {
		label = label[:8]
	}
	raw, err := h.opt.Mint(c.Request().Context(), userID, ApprovalMintRequest(label, GrantScope{
		ProjectGrantMode:  body.ProjectGrantMode,
		ProjectIDs:        body.ProjectIDs,
		TenantGrantMode:   body.TenantGrantMode,
		TenantIDs:         body.TenantIDs,
		AcknowledgeDanger: body.AcknowledgeDanger,
	}))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": err.Error()})
	}
	code := randomToken(32)
	if err := h.store.SaveCode(code, CodeRecord{
		ClientID: rec.ClientID, RedirectURI: rec.RedirectURI, Challenge: rec.CodeChallenge,
		Method: rec.Method, Resource: rec.Resource, UserID: userID, InternalToken: raw,
		ProjectMode: body.ProjectGrantMode, ProjectIDs: body.ProjectIDs,
		TenantMode: body.TenantGrantMode, TenantIDs: body.TenantIDs,
		ExpiresAt: h.now().Add(codeTTL),
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "could not store authorization code"})
	}
	rec.Status = "approved"
	_ = h.store.SaveRequest(rec)
	target, err := url.Parse(rec.RedirectURI)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "invalid redirect"})
	}
	q := target.Query()
	q.Set("code", code)
	q.Set("iss", h.issuer(c))
	if rec.State != "" {
		q.Set("state", rec.State)
	}
	target.RawQuery = q.Encode()
	return c.JSON(http.StatusOK, map[string]string{"status": "approved", "redirect_to": target.String()})
}

func (h *Handler) token(c echo.Context) error {
	if !h.allow(c, 60) {
		return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "slow_down"})
	}
	raw, _ := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	c.Set("oauth_body", raw)
	if len(raw) > 0 && raw[0] != '{' {
		c.Request().Body = io.NopCloser(strings.NewReader(string(raw)))
		_ = c.Request().ParseForm()
	}
	grant := formValue(c, "grant_type")
	switch grant {
	case "authorization_code":
		return h.authorizationCode(c)
	case "refresh_token":
		return h.refresh(c)
	case "urn:ietf:params:oauth:grant-type:token-exchange":
		return h.exchange(c)
	default:
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "unsupported_grant_type"})
	}
}

func (h *Handler) authorizationCode(c echo.Context) error {
	code := formValue(c, "code")
	verifier := formValue(c, "code_verifier")
	clientID := formValue(c, "client_id")
	redirectURI := formValue(c, "redirect_uri")
	resource, err := CanonicalResource(formValue(c, "resource"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_target"})
	}
	rec, ok := h.store.TakeCode(code)
	if !ok {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
	}
	if rec.ClientID != clientID || rec.RedirectURI != redirectURI || rec.Resource != resource || rec.Resource != h.resource(c) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
	}
	if !verifyS256(verifier, rec.Challenge) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
	}
	return h.issue(c, rec.ClientID, rec.Resource, rec.UserID, rec.InternalToken)
}

func (h *Handler) refresh(c echo.Context) error {
	raw := formValue(c, "refresh_token")
	resource, err := CanonicalResource(formValue(c, "resource"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_target"})
	}
	rec, ok := h.store.TakeRefresh(raw)
	if !ok || rec.ClientID != formValue(c, "client_id") || rec.Resource != resource {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
	}
	return h.issue(c, rec.ClientID, rec.Resource, rec.UserID, rec.InternalToken)
}

func (h *Handler) exchange(c echo.Context) error {
	subject := formValue(c, "subject_token")
	if !strings.HasPrefix(subject, models.AccessTokenPrefix) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_grant"})
	}
	resource := formValue(c, "resource")
	if resource == "" {
		resource = h.resource(c)
	}
	resource, err := CanonicalResource(resource)
	if err != nil || resource != h.resource(c) {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_target"})
	}
	return h.issue(c, "apito-cli", resource, "", subject)
}

func (h *Handler) issue(c echo.Context, clientID, resource, userID, internal string) error {
	access := "mct_" + randomToken(32)
	refresh := "mcr_" + randomToken(32)
	if err := h.store.SaveAccess(access, AccessRecord{
		ClientID: clientID, Resource: resource, UserID: userID, InternalToken: internal,
		Scope: ScopeMCP, ExpiresAt: h.now().Add(accessTTL),
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "server_error"})
	}
	if err := h.store.SaveRefresh(refresh, RefreshRecord{
		ClientID: clientID, Resource: resource, UserID: userID, InternalToken: internal,
		Scope: ScopeMCP, ExpiresAt: h.now().Add(refreshTTL),
	}); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "server_error"})
	}
	return c.JSON(http.StatusOK, map[string]any{
		"access_token":  access,
		"token_type":    "Bearer",
		"expires_in":    int(accessTTL.Seconds()),
		"refresh_token": refresh,
		"scope":         ScopeMCP,
		"resource":      resource,
	})
}

func (h *Handler) mcp(c echo.Context) error {
	h.cors(c)
	if c.Request().Method == http.MethodOptions {
		return c.NoContent(http.StatusNoContent)
	}
	if !originAllowed(c, h.resource(c)) {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "invalid_origin"})
	}
	if c.Request().Method == http.MethodDelete {
		if id := c.Request().Header.Get("Mcp-Session-Id"); id != "" {
			h.sessions.Delete(id)
		}
		return c.NoContent(http.StatusNoContent)
	}
	if c.Request().URL.RawQuery != "" {
		if _, err := CanonicalResource(h.resource(c)); err != nil {
			return h.unauthorized(c)
		}
	}
	token := bearer(c)
	if token == "" || strings.HasPrefix(token, models.AccessTokenPrefix) || strings.Contains(c.Request().URL.RawQuery, "api_key=") {
		return h.unauthorized(c)
	}
	access, ok := h.store.Access(token)
	if !ok || access.Resource != h.resource(c) {
		return h.unauthorized(c)
	}
	if c.Request().Method == http.MethodGet {
		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().WriteHeader(http.StatusOK)
		_, _ = c.Response().Write([]byte(": open\n\n"))
		return nil
	}
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid_request"})
	}
	var req jsonRPC
	if err := json.Unmarshal(body, &req); err != nil {
		return h.writeRPC(c, jsonRPC{Error: &rpcError{Code: -32700, Message: "parse error"}})
	}
	session := c.Request().Header.Get("Mcp-Session-Id")
	if session == "" {
		session = randomToken(12)
	}
	h.sessions.Store(session, access.Resource)
	c.Response().Header().Set("Mcp-Session-Id", session)
	switch req.Method {
	case "initialize":
		return h.writeRPC(c, result(req.ID, map[string]any{
			"protocolVersion": mcpProtocol,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "apito-mcp", "version": "2.0.0"},
		}))
	case "notifications/initialized", "ping":
		if req.ID == nil {
			return c.NoContent(http.StatusAccepted)
		}
		return h.writeRPC(c, result(req.ID, map[string]any{}))
	case "tools/list":
		return h.writeRPC(c, result(req.ID, map[string]any{"tools": h.toolList()}))
	case "tools/call":
		return h.writeRPC(c, h.call(c, req, access))
	default:
		return h.writeRPC(c, jsonRPC{ID: req.ID, Error: &rpcError{Code: -32601, Message: "method not found"}})
	}
}

func (h *Handler) toolList() []map[string]any {
	edition := h.opt.Edition
	out := []map[string]any{}
	for _, tool := range Catalog() {
		if edition == "open" && tool.ProOnly {
			continue
		}
		out = append(out, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.schema(),
		})
	}
	return out
}

func (h *Handler) call(c echo.Context, req jsonRPC, access AccessRecord) jsonRPC {
	var params struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	_ = json.Unmarshal(req.Params, &params)
	tool, ok := findTool(params.Name)
	if !ok || (h.opt.Edition == "open" && tool.ProOnly) {
		return jsonRPC{ID: req.ID, Error: &rpcError{Code: -32602, Message: "unknown tool"}}
	}
	args := aliasArgs(params.Name, params.Arguments)
	projectID, _ := args["project_id"].(string)
	tenantID, _ := args["tenant_id"].(string)
	audit := AuditEvent{RequestID: fmt.Sprint(req.ID), Tool: params.Name, Transport: "http", Edition: h.opt.Edition, ProjectID: projectID, TenantID: tenantID}
	if text, handled, err := h.localTool(params.Name, args); handled {
		audit.ScopeOK = err == nil
		audit.Error = errString(err)
		h.audit(c.Request().Context(), audit)
		if err != nil {
			return toolError(req.ID, err)
		}
		return toolText(req.ID, text)
	}
	if err := h.enforce(tool, args); err != nil {
		audit.Error = err.Error()
		h.audit(c.Request().Context(), audit)
		return toolError(req.ID, err)
	}
	audit.ScopeOK = true
	if params.Name == "inspect_access_token" && h.opt.System != nil {
		_, raw, err := h.opt.System.REST(c.Request().Context(), access.InternalToken, http.MethodGet, "/system/access-tokens/me", nil)
		audit.Error = errString(err)
		h.audit(c.Request().Context(), audit)
		if err != nil {
			return toolError(req.ID, err)
		}
		return toolText(req.ID, string(raw))
	}
	query, mapped := operationQuery(params.Name)
	if !mapped {
		err := fmt.Errorf("tool %s is registered but has no system operation mapping", params.Name)
		audit.Error = err.Error()
		h.audit(c.Request().Context(), audit)
		return toolError(req.ID, err)
	}
	if h.opt.System == nil {
		err := fmt.Errorf("system graphql is not configured")
		audit.Error = err.Error()
		h.audit(c.Request().Context(), audit)
		return toolError(req.ID, err)
	}
	raw, err := h.opt.System.GraphQL(c.Request().Context(), access.InternalToken, projectID, tenantID, query, variablesFor(query, args))
	audit.Error = errString(err)
	h.audit(c.Request().Context(), audit)
	if err != nil {
		return toolError(req.ID, err)
	}
	return toolText(req.ID, string(raw))
}

func (h *Handler) localTool(name string, args map[string]any) (string, bool, error) {
	if text, ok := guides()[name]; ok && name != "inspect_access_token" {
		return text, true, nil
	}
	switch name {
	case "prepare_project_scope":
		id := randomToken(16)
		projectID, _ := args["project_id"].(string)
		h.leases.Store("prep:"+id, lease{ProjectID: projectID, Expires: h.now().Add(30 * time.Minute)})
		raw, _ := json.Marshal(map[string]string{"preparation_id": id})
		return string(raw), true, nil
	case "confirm_project_scope":
		prep, _ := args["preparation_id"].(string)
		value, ok := h.leases.Load("prep:" + prep)
		if !ok {
			return "", true, fmt.Errorf("preparation is expired")
		}
		item := value.(lease)
		if h.now().After(item.Expires) {
			return "", true, fmt.Errorf("preparation is expired")
		}
		projectID, _ := args["project_id"].(string)
		if projectID == "" {
			projectID = item.ProjectID
		}
		leaseID := randomToken(16)
		tenantID, _ := args["tenant_id"].(string)
		h.leases.Store("lease:"+leaseID, lease{ProjectID: projectID, TenantID: tenantID, Expires: h.now().Add(30 * time.Minute)})
		raw, _ := json.Marshal(map[string]string{"scope_lease": leaseID, "project_id": projectID})
		return string(raw), true, nil
	case "get_project_scope":
		raw, _ := json.Marshal(map[string]string{"status": "explicit project scope required"})
		return string(raw), true, nil
	default:
		return "", false, nil
	}
}

func (h *Handler) enforce(tool Tool, args map[string]any) error {
	if tool.Access == accessGuide || tool.Access == accessScope {
		return nil
	}
	if tool.Access == accessDestructive {
		if args["confirm_destructive"] != true {
			return fmt.Errorf("confirm_destructive must be true")
		}
		if tool.Name == "delete_model" && args["acknowledge_permanent_deletion"] != true {
			return fmt.Errorf("acknowledge_permanent_deletion must be true")
		}
	}
	if tool.Access == accessWrite || tool.Access == accessDestructive {
		leaseID, _ := args["scope_lease"].(string)
		value, ok := h.leases.Load("lease:" + leaseID)
		if !ok {
			return fmt.Errorf("scope_lease is required")
		}
		item := value.(lease)
		projectID, _ := args["project_id"].(string)
		if h.now().After(item.Expires) || (projectID != "" && item.ProjectID != "" && item.ProjectID != projectID) {
			return fmt.Errorf("scope_lease does not match this project")
		}
	}
	return nil
}

func (h *Handler) audit(ctx context.Context, event AuditEvent) {
	if h.opt.Audit != nil {
		h.opt.Audit.Record(ctx, event)
	}
}

func (h *Handler) unauthorized(c echo.Context) error {
	metadata := h.issuer(c) + "/.well-known/oauth-protected-resource/mcp"
	c.Response().Header().Set("WWW-Authenticate", `Bearer realm="apito-mcp", resource_metadata="`+metadata+`"`)
	return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid_token"})
}

func (h *Handler) cors(c echo.Context) {
	origin := c.Request().Header.Get("Origin")
	if origin == "" || !originAllowed(c, h.resource(c)) {
		return
	}
	c.Response().Header().Set("Access-Control-Allow-Origin", origin)
	c.Response().Header().Set("Vary", "Origin")
	c.Response().Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept, MCP-Protocol-Version, Mcp-Session-Id, Last-Event-ID")
	c.Response().Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
	c.Response().Header().Set("Access-Control-Expose-Headers", "WWW-Authenticate, Mcp-Session-Id, MCP-Protocol-Version")
}

func (h *Handler) allow(c echo.Context, limit int) bool {
	ip, _, _ := net.SplitHostPort(c.Request().RemoteAddr)
	if ip == "" {
		ip = c.RealIP()
	}
	key := ip + h.now().Format("200601021504")
	value, _ := h.limits.LoadOrStore(key, 0)
	count, _ := value.(int)
	count++
	h.limits.Store(key, count)
	return count <= limit
}

func (h *Handler) writeRPC(c echo.Context, msg jsonRPC) error {
	msg.JSONRPC = "2.0"
	accept := c.Request().Header.Get("Accept")
	if strings.Contains(accept, "text/event-stream") && !strings.Contains(accept, "application/json") {
		raw, _ := json.Marshal(msg)
		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().WriteHeader(http.StatusOK)
		_, _ = c.Response().Write([]byte("event: message\ndata: " + string(raw) + "\n\n"))
		return nil
	}
	status := http.StatusOK
	if msg.Error != nil && msg.Method == "" && msg.Result == nil && msg.ID == nil {
		status = http.StatusBadRequest
	}
	return c.JSON(status, msg)
}

type jsonRPC struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func result(id any, value any) jsonRPC {
	return jsonRPC{ID: id, Result: value}
}

func toolText(id any, text string) jsonRPC {
	return result(id, map[string]any{
		"content": []map[string]string{{"type": "text", "text": text}},
	})
}

func toolError(id any, err error) jsonRPC {
	return jsonRPC{ID: id, Error: &rpcError{Code: -32000, Message: err.Error()}}
}

func findTool(name string) (Tool, bool) {
	for _, tool := range Catalog() {
		if tool.Name == name {
			return tool, true
		}
	}
	return Tool{}, false
}

func bearer(c echo.Context) string {
	value := c.Request().Header.Get("Authorization")
	if strings.HasPrefix(strings.ToLower(value), "bearer ") {
		return strings.TrimSpace(value[7:])
	}
	return ""
}

func formValue(c echo.Context, key string) string {
	if value := c.FormValue(key); value != "" {
		return value
	}
	if raw, ok := c.Get("oauth_body").([]byte); ok && len(raw) > 0 && raw[0] == '{' {
		var body map[string]any
		if json.Unmarshal(raw, &body) == nil {
			if value, ok := body[key].(string); ok {
				return value
			}
		}
	}
	return c.QueryParam(key)
}

func validateRedirect(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Fragment != "" {
		return fmt.Errorf("invalid redirect")
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && isLoopback(parsed.Hostname()) {
		return nil
	}
	return fmt.Errorf("invalid redirect")
}

func exactRedirect(allowed []string, raw string) bool {
	for _, item := range allowed {
		if item == raw {
			return true
		}
	}
	return false
}

func isLoopback(host string) bool {
	host = strings.Trim(host, "[]")
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func originAllowed(c echo.Context, resource string) bool {
	origin := c.Request().Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	resourceURL, err := url.Parse(resource)
	if err != nil {
		return false
	}
	if strings.EqualFold(parsed.Host, resourceURL.Host) {
		return true
	}
	return isLoopback(parsed.Hostname()) && isLoopback(resourceURL.Hostname())
}

func verifyS256(verifier, challenge string) bool {
	if len(verifier) < 43 || len(verifier) > 128 {
		return false
	}
	sum := sha256.Sum256([]byte(verifier))
	got := base64.RawURLEncoding.EncodeToString(sum[:])
	if len(got) != len(challenge) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(challenge)) == 1
}

func randomToken(bytes int) string {
	buf := make([]byte, bytes)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

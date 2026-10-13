package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

func TestOAuthAndMCPHTTP(t *testing.T) {
	e := echo.New()
	system := &recordingSystem{}
	var audited []AuditEvent
	_, err := Mount(e, Options{
		Config:    &models.Config{MCPPublicURL: "http://example.com", CORSOrigin: "http://localhost:4055"},
		StorePath: filepath.Join(t.TempDir(), "oauth.json"),
		Authorize: func(next echo.HandlerFunc) echo.HandlerFunc {
			return func(c echo.Context) error {
				c.Set("user", "user-1")
				return next(c)
			}
		},
		Mint: func(_ context.Context, userID string, req *models.CreateAccessTokenRequest) (string, error) {
			if userID != "user-1" || req.Preset != "mcp_assistant" {
				t.Fatalf("mint user=%s preset=%s", userID, req.Preset)
			}
			return "apt_internal", nil
		},
		System: system,
		Audit:  auditFunc(func(event AuditEvent) { audited = append(audited, event) }),
	})
	if err != nil {
		t.Fatal(err)
	}

	verifier := strings.Repeat("a", 43)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	reg := postJSON(t, e, "/oauth/register", `{"client_name":"Cursor","redirect_uris":["http://127.0.0.1:9999/callback"],"token_endpoint_auth_method":"none"}`, "")
	if reg.Code != http.StatusCreated {
		t.Fatalf("register %d %s", reg.Code, reg.Body.String())
	}
	var client map[string]any
	_ = json.Unmarshal(reg.Body.Bytes(), &client)
	clientID := client["client_id"].(string)

	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", "http://127.0.0.1:9999/callback")
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	q.Set("resource", "http://example.com/mcp")
	q.Set("state", "state-1")
	q.Set("scope", "mcp")
	auth := request(t, e, http.MethodGet, "/oauth/authorize?"+q.Encode(), "", "")
	if auth.Code != http.StatusFound || !strings.Contains(auth.Header().Get("Location"), "http://localhost:4055/mcp?request_id=") {
		t.Fatalf("authorize %d %s", auth.Code, auth.Header().Get("Location"))
	}
	location, _ := url.Parse(auth.Header().Get("Location"))
	requestID := location.Query().Get("request_id")

	approved := postJSON(t, e, "/system/oauth/mcp/approve", `{"request_id":"`+requestID+`","project_grant_mode":"selected","project_ids":["p1"],"tenant_grant_mode":"none"}`, "")
	if approved.Code != http.StatusOK || strings.Contains(approved.Body.String(), "apt_") {
		t.Fatalf("approve %d %s", approved.Code, approved.Body.String())
	}
	var approval map[string]string
	_ = json.Unmarshal(approved.Body.Bytes(), &approval)
	redirect, _ := url.Parse(approval["redirect_to"])
	if redirect.Query().Get("state") != "state-1" || redirect.Query().Get("iss") == "" {
		t.Fatalf("redirect = %s", approval["redirect_to"])
	}
	code := redirect.Query().Get("code")

	bad := postForm(t, e, "/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {strings.Repeat("b", 43)},
		"client_id": {clientID}, "redirect_uri": {"http://127.0.0.1:9999/callback"}, "resource": {"http://example.com/mcp"},
	})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad verifier %d", bad.Code)
	}
	replay := postForm(t, e, "/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {verifier},
		"client_id": {clientID}, "redirect_uri": {"http://127.0.0.1:9999/callback"}, "resource": {"http://example.com/mcp"},
	})
	if replay.Code != http.StatusBadRequest {
		t.Fatalf("replayed code %d %s", replay.Code, replay.Body.String())
	}

	auth = request(t, e, http.MethodGet, "/oauth/authorize?"+q.Encode(), "", "")
	location, _ = url.Parse(auth.Header().Get("Location"))
	requestID = location.Query().Get("request_id")
	approved = postJSON(t, e, "/system/oauth/mcp/approve", `{"request_id":"`+requestID+`","project_grant_mode":"selected","project_ids":["p1"],"tenant_grant_mode":"none"}`, "")
	_ = json.Unmarshal(approved.Body.Bytes(), &approval)
	redirect, _ = url.Parse(approval["redirect_to"])
	code = redirect.Query().Get("code")
	okToken := postForm(t, e, "/oauth/token", url.Values{
		"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {verifier},
		"client_id": {clientID}, "redirect_uri": {"http://127.0.0.1:9999/callback"}, "resource": {"http://example.com/mcp"},
	})
	if okToken.Code != http.StatusOK || strings.Contains(okToken.Body.String(), "apt_") {
		t.Fatalf("token %d %s", okToken.Code, okToken.Body.String())
	}
	var token map[string]any
	_ = json.Unmarshal(okToken.Body.Bytes(), &token)
	access := token["access_token"].(string)
	if !strings.HasPrefix(access, "mct_") {
		t.Fatalf("access = %s", access)
	}

	unauth := postJSON(t, e, "/mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, "apt_internal")
	if unauth.Code != http.StatusUnauthorized || !strings.Contains(unauth.Header().Get("WWW-Authenticate"), "oauth-protected-resource") {
		t.Fatalf("apt bearer %d %s", unauth.Code, unauth.Header().Get("WWW-Authenticate"))
	}
	init := postJSON(t, e, "/mcp", `{"jsonrpc":"2.0","id":1,"method":"initialize"}`, access)
	if init.Code != http.StatusOK || !strings.Contains(init.Body.String(), "apito-mcp") {
		t.Fatalf("initialize %d %s", init.Code, init.Body.String())
	}
	list := postJSON(t, e, "/mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`, access)
	if list.Code != http.StatusOK || !strings.Contains(list.Body.String(), "search_tenants") {
		t.Fatalf("tools/list %s", list.Body.String())
	}
	denied := postJSON(t, e, "/mcp", `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"delete_tenant","arguments":{"tenant_id":"t1"}}}`, access)
	if !strings.Contains(denied.Body.String(), "confirm_destructive") {
		t.Fatalf("destructive = %s", denied.Body.String())
	}
	called := postJSON(t, e, "/mcp", `{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"search_tenants","arguments":{"project_id":"p1","q":"acme"}}}`, access)
	if !strings.Contains(system.query, "searchTenants") || !strings.Contains(called.Body.String(), "ok") {
		t.Fatalf("query %s body %s", system.query, called.Body.String())
	}
	if len(audited) == 0 || audited[len(audited)-1].Tool != "search_tenants" {
		t.Fatalf("audit = %+v", audited)
	}

	meta := request(t, e, http.MethodGet, "/.well-known/oauth-protected-resource/mcp", "", "")
	if !strings.Contains(meta.Body.String(), `"resource":"http://example.com/mcp"`) {
		t.Fatalf("metadata %s", meta.Body.String())
	}
}

func TestOpenEditionHidesProTools(t *testing.T) {
	e := echo.New()
	_, err := Mount(e, Options{
		Config:    &models.Config{MCPPublicURL: "http://example.com"},
		StorePath: filepath.Join(t.TempDir(), "oauth.json"),
		Edition:   "open",
		Mint: func(_ context.Context, userID string, req *models.CreateAccessTokenRequest) (string, error) {
			return "apt_internal", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// Direct list through a minted access record is covered by the handler tool list.
	h := &Handler{opt: Options{Edition: "open"}}
	raw, _ := json.Marshal(h.toolList())
	if strings.Contains(string(raw), "search_tenants") || strings.Contains(string(raw), "get_log_store_health") {
		t.Fatalf("open edition listed pro tools: %s", raw)
	}
	if !strings.Contains(string(raw), "list_models") {
		t.Fatalf("open edition dropped a public tool")
	}
	_ = e
}

func TestCatalogCoversTypeScriptSurface(t *testing.T) {
	if got := len(Catalog()); got < 100 {
		t.Fatalf("catalog = %d", got)
	}
	seen := map[string]bool{}
	for _, tool := range Catalog() {
		if seen[tool.Name] {
			t.Fatalf("duplicate %s", tool.Name)
		}
		seen[tool.Name] = true
	}
	for _, name := range []string{"prepare_project_scope", "create_model", "get_data", "search_tenants", "execute_function", "get_log_store_health"} {
		if !seen[name] {
			t.Fatalf("missing %s", name)
		}
	}
}

type recordingSystem struct {
	query string
}

func (r *recordingSystem) GraphQL(_ context.Context, token, projectID, tenantID, query string, vars map[string]any) (json.RawMessage, error) {
	r.query = query
	if token != "apt_internal" || projectID != "p1" {
		return json.RawMessage(`{"error":"mismatch"}`), nil
	}
	return json.RawMessage(`{"data":{"ok":true}}`), nil
}

func (r *recordingSystem) REST(_ context.Context, token, method, path string, body []byte) (int, json.RawMessage, error) {
	return http.StatusNotFound, nil, nil
}

type auditFunc func(AuditEvent)

func (f auditFunc) Record(_ context.Context, event AuditEvent) {
	f(event)
}

func postJSON(t *testing.T, e *echo.Echo, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	return request(t, e, http.MethodPost, path, body, token)
}

func postForm(t *testing.T, e *echo.Echo, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func request(t *testing.T, e *echo.Echo, method, path, body, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Host = "example.com"
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

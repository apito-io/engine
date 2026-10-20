package controller

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/mcp"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

const (
	deviceAudienceCLI = "cli"
	deviceAudienceMCP = "mcp"
)

type deviceLoginRecord struct {
	Audience         string
	DeviceCode       string
	UserCode         string
	Expires          time.Time
	Interval         int
	Status           string
	Token            string
	ConsoleURL       string
	ProjectGrantMode string
	ProjectIDs       []string
	TenantGrantMode  string
	TenantIDs        map[string][]string
}

var deviceLogins sync.Map

func (a *AuthController) CLILoginStart(c echo.Context) error {
	return a.deviceLoginStart(c, deviceAudienceCLI)
}

func (a *AuthController) MCPLoginStart(c echo.Context) error {
	return a.deviceLoginStart(c, deviceAudienceMCP)
}

func (a *AuthController) deviceLoginStart(c echo.Context, audience string) error {
	deviceRaw := make([]byte, 32)
	userRaw := make([]byte, 4)
	if _, err := rand.Read(deviceRaw); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "could not start login"})
	}
	if _, err := rand.Read(userRaw); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "could not start login"})
	}
	deviceCode := hex.EncodeToString(deviceRaw)
	userCode := strings.ToUpper(hex.EncodeToString(userRaw))
	deviceLogins.Store(deviceCode, &deviceLoginRecord{
		Audience:   audience,
		DeviceCode: deviceCode,
		UserCode:   userCode,
		Expires:    time.Now().Add(10 * time.Minute),
		Interval:   5,
		Status:     "pending",
	})
	return c.JSON(http.StatusOK, map[string]interface{}{
		"device_code":      deviceCode,
		"user_code":        userCode,
		"verification_uri": a.deviceLoginPageURL(c, audience, deviceCode, userCode),
		"interval":         5,
		"expires_in":       600,
	})
}

func (a *AuthController) CLILoginPoll(c echo.Context) error {
	return a.deviceLoginPoll(c, deviceAudienceCLI)
}

func (a *AuthController) MCPLoginPoll(c echo.Context) error {
	return a.deviceLoginPoll(c, deviceAudienceMCP)
}

func (a *AuthController) deviceLoginPoll(c echo.Context, audience string) error {
	var body struct {
		DeviceCode string `json:"device_code" form:"device_code"`
	}
	if err := c.Bind(&body); err != nil || strings.TrimSpace(body.DeviceCode) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "device_code is required"})
	}
	rec, ok := loadDeviceLogin(body.DeviceCode, audience)
	if !ok {
		return c.JSON(http.StatusOK, map[string]string{"status": "expired"})
	}
	if time.Now().After(rec.Expires) && rec.Status == "pending" {
		rec.Status = "expired"
		deviceLogins.Store(rec.DeviceCode, rec)
	}
	resp := map[string]interface{}{"status": rec.Status, "console_url": rec.ConsoleURL}
	if rec.Status == "approved" && rec.Token != "" {
		resp["access_token"] = rec.Token
		resp["project_grant_mode"] = rec.ProjectGrantMode
		resp["project_ids"] = rec.ProjectIDs
		resp["tenant_grant_mode"] = rec.TenantGrantMode
		resp["tenant_ids"] = rec.TenantIDs
		rec.Token = ""
		rec.Status = "consumed"
		deviceLogins.Store(rec.DeviceCode, rec)
	}
	return c.JSON(http.StatusOK, resp)
}

func (a *AuthController) CLILoginApprove(c echo.Context) error {
	return a.deviceLoginApprove(c, deviceAudienceCLI)
}

func (a *AuthController) MCPLoginApprove(c echo.Context) error {
	return a.deviceLoginApprove(c, deviceAudienceMCP)
}

func (a *AuthController) deviceLoginApprove(c echo.Context, audience string) error {
	userID, _ := c.Get("user").(string)
	if strings.TrimSpace(userID) == "" {
		message := "sign in to the console before approving the CLI"
		if audience == deviceAudienceMCP {
			message = "sign in to the console before approving MCP"
		}
		return c.JSON(http.StatusUnauthorized, map[string]string{"message": message})
	}
	deviceCode, scope, err := cliLoginApprovalFromRequest(c)
	if err != nil || deviceCode == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "device_code is required"})
	}
	rec, ok := loadDeviceLogin(deviceCode, audience)
	if !ok || time.Now().After(rec.Expires) || rec.Status != "pending" {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": "device code expired"})
	}
	svc := a.accessTokenSvc()
	if svc == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"message": "access token service unavailable"})
	}
	mintReq, err := deviceLoginMintRequest(audience, rec.UserCode, scope)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": err.Error()})
	}
	raw, _, err := svc.Mint(c.Request().Context(), userID, mintReq)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"message": err.Error()})
	}
	rec.Status = "approved"
	rec.Token = raw
	rec.ProjectGrantMode = scope.ProjectGrantMode
	rec.ProjectIDs = scope.ProjectIDs
	rec.TenantGrantMode = scope.TenantGrantMode
	rec.TenantIDs = scope.TenantIDs
	if a.Cfg != nil {
		rec.ConsoleURL = strings.TrimSpace(a.Cfg.CORSOrigin)
	}
	deviceLogins.Store(rec.DeviceCode, rec)
	return c.JSON(http.StatusOK, map[string]string{"status": "approved"})
}

func (a *AuthController) deviceLoginPageURL(c echo.Context, audience, deviceCode, userCode string) string {
	origin := ""
	if a != nil && a.Cfg != nil {
		origin = strings.TrimRight(strings.TrimSpace(a.Cfg.CORSOrigin), "/")
	}
	if origin == "" {
		scheme := "http"
		if c.Request().TLS != nil || strings.EqualFold(c.Request().Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		origin = scheme + "://" + c.Request().Host
	}
	q := url.Values{}
	q.Set("device_code", deviceCode)
	if strings.TrimSpace(userCode) != "" {
		q.Set("user_code", userCode)
	}
	return origin + deviceLoginConsolePath(audience) + "?" + q.Encode()
}

func deviceLoginConsolePath(audience string) string {
	if audience == deviceAudienceMCP {
		return "/mcp"
	}
	return "/cli"
}

func (a *AuthController) CLILoginVerify(c echo.Context) error {
	return a.deviceLoginVerify(c, deviceAudienceCLI)
}

func (a *AuthController) MCPLoginVerify(c echo.Context) error {
	return a.deviceLoginVerify(c, deviceAudienceMCP)
}

func (a *AuthController) deviceLoginVerify(c echo.Context, audience string) error {
	code := strings.TrimSpace(c.QueryParam("device_code"))
	userCode := strings.TrimSpace(c.QueryParam("user_code"))
	if rec, ok := loadDeviceLogin(code, audience); ok && userCode == "" {
		userCode = rec.UserCode
	}
	return c.Redirect(http.StatusFound, a.deviceLoginPageURL(c, audience, code, userCode))
}

type cliLoginScope struct {
	Name              string
	Description       string
	Duration          string
	Preset            string
	Capabilities      []string
	ProjectGrantMode  string
	ProjectIDs        []string
	TenantGrantMode   string
	TenantIDs         map[string][]string
	AcknowledgeDanger bool
}

func cliLoginApprovalFromRequest(c echo.Context) (string, cliLoginScope, error) {
	raw, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return "", cliLoginScope{}, err
	}
	scope := cliLoginScope{
		ProjectGrantMode: models.AccessTokenProjectGrantSelected,
		TenantGrantMode:  models.AccessTokenTenantGrantNone,
		TenantIDs:        map[string][]string{},
	}
	contentType := c.Request().Header.Get(echo.HeaderContentType)
	if strings.Contains(contentType, echo.MIMEApplicationJSON) || (len(bytes.TrimSpace(raw)) > 0 && raw[0] == '{') {
		var body struct {
			DeviceCode        string              `json:"device_code"`
			Name              string              `json:"name"`
			Description       string              `json:"description"`
			Duration          string              `json:"duration"`
			Preset            string              `json:"preset"`
			Capabilities      []string            `json:"capabilities"`
			ProjectGrantMode  string              `json:"project_grant_mode"`
			ProjectIDs        []string            `json:"project_ids"`
			TenantGrantMode   string              `json:"tenant_grant_mode"`
			TenantIDs         map[string][]string `json:"tenant_ids"`
			AcknowledgeDanger bool                `json:"acknowledge_danger"`
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			return "", cliLoginScope{}, err
		}
		if body.ProjectGrantMode != "" {
			scope.ProjectGrantMode = body.ProjectGrantMode
		}
		if body.TenantGrantMode != "" {
			scope.TenantGrantMode = body.TenantGrantMode
		}
		scope.Name = body.Name
		scope.Description = body.Description
		scope.Duration = body.Duration
		scope.Preset = body.Preset
		scope.Capabilities = body.Capabilities
		scope.ProjectIDs = body.ProjectIDs
		if body.TenantIDs != nil {
			scope.TenantIDs = body.TenantIDs
		}
		scope.AcknowledgeDanger = body.AcknowledgeDanger
		return strings.TrimSpace(body.DeviceCode), scope, nil
	}
	values, err := url.ParseQuery(string(raw))
	if err != nil {
		return "", cliLoginScope{}, err
	}
	if mode := strings.TrimSpace(values.Get("project_grant_mode")); mode != "" {
		scope.ProjectGrantMode = mode
	}
	if mode := strings.TrimSpace(values.Get("tenant_grant_mode")); mode != "" {
		scope.TenantGrantMode = mode
	}
	scope.ProjectIDs = values["project_ids"]
	scope.AcknowledgeDanger = values.Get("acknowledge_danger") == "true" || values.Get("acknowledge_danger") == "on"
	for _, pair := range values["tenant_ids"] {
		projectID, tenantID, ok := strings.Cut(pair, "|")
		if ok && strings.TrimSpace(projectID) != "" && strings.TrimSpace(tenantID) != "" {
			scope.TenantIDs[strings.TrimSpace(projectID)] = append(scope.TenantIDs[strings.TrimSpace(projectID)], strings.TrimSpace(tenantID))
		}
	}
	for _, line := range strings.Split(values.Get("tenant_pairs"), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		scope.TenantIDs[fields[0]] = append(scope.TenantIDs[fields[0]], fields[1])
	}
	if scope.TenantGrantMode != models.AccessTokenTenantGrantSelected {
		scope.TenantIDs = nil
	}
	if scope.ProjectGrantMode != models.AccessTokenProjectGrantSelected {
		scope.ProjectIDs = nil
	}
	return strings.TrimSpace(values.Get("device_code")), scope, nil
}

// cliLoginMintRequest uses the low-harm CLI login preset and the scope chosen
// on the approval page. Capabilities stay on the preset. Mint is the same call
// the console uses to create an access token. Sync and function deploy stay on
// an explicit cli_sync token.
func cliLoginMintRequest(userCode string, scope cliLoginScope) (*models.CreateAccessTokenRequest, error) {
	return deviceLoginMintRequest(deviceAudienceCLI, userCode, scope)
}

func deviceLoginMintRequest(audience, userCode string, scope cliLoginScope) (*models.CreateAccessTokenRequest, error) {
	if audience == deviceAudienceMCP {
		return mcp.ApprovalMintRequest(userCode, mcp.GrantScope{
			Name:              scope.Name,
			Description:       scope.Description,
			Duration:          scope.Duration,
			Preset:            scope.Preset,
			Capabilities:      scope.Capabilities,
			ProjectGrantMode:  scope.ProjectGrantMode,
			ProjectIDs:        scope.ProjectIDs,
			TenantGrantMode:   scope.TenantGrantMode,
			TenantIDs:         scope.TenantIDs,
			AcknowledgeDanger: scope.AcknowledgeDanger,
		})
	}
	presetID, caps, err := normalizeCLIPreset(scope.Preset, scope.Capabilities)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(scope.Name)
	if name == "" {
		name = "CLI " + strings.TrimSpace(userCode)
	}
	if strings.TrimSpace(name) == "" || name == "CLI" {
		name = "CLI login"
	}
	if len(name) > 120 {
		name = name[:120]
	}
	description := strings.TrimSpace(scope.Description)
	if len(description) > 500 {
		description = description[:500]
	}
	preset := deviceLoginPreset(presetID, "CLI login")
	if scope.ProjectGrantMode == "" {
		scope.ProjectGrantMode = models.AccessTokenProjectGrantSelected
	}
	if scope.TenantGrantMode == "" {
		scope.TenantGrantMode = models.AccessTokenTenantGrantNone
	}
	projectIDs := scope.ProjectIDs
	if scope.ProjectGrantMode != models.AccessTokenProjectGrantSelected {
		projectIDs = nil
	}
	tenantIDs := scope.TenantIDs
	if scope.TenantGrantMode != models.AccessTokenTenantGrantSelected {
		tenantIDs = nil
	}
	return &models.CreateAccessTokenRequest{
		Name:              name,
		Description:       description,
		Duration:          strings.TrimSpace(scope.Duration),
		Preset:            preset.ID,
		Capabilities:      caps,
		ProjectGrantMode:  scope.ProjectGrantMode,
		ProjectIDs:        projectIDs,
		TenantGrantMode:   scope.TenantGrantMode,
		TenantIDs:         tenantIDs,
		AcknowledgeDanger: scope.AcknowledgeDanger,
	}, nil
}

func normalizeCLIPreset(raw string, caps []string) (string, []string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "cli_login":
		return "cli_login", nil, nil
	case "cli_sync":
		return "cli_sync", nil, nil
	case "custom":
		if len(caps) == 0 {
			return "", nil, errors.New("custom preset requires capabilities")
		}
		return "custom", append([]string(nil), caps...), nil
	default:
		return "", nil, errors.New("choose CLI login, CLI sync, or Custom")
	}
}

func deviceLoginPreset(id, fallback string) authz.Preset {
	for _, preset := range authz.Presets() {
		if preset.ID == id {
			return preset
		}
	}
	return authz.Preset{ID: id, Label: fallback}
}

func (a *AuthController) CLICapabilities(c echo.Context) error {
	flags := map[string]bool{
		"project_access":          true,
		"project_tokens":          true,
		"published_introspection": true,
		"mcp":                     true,
		"cli_login":               true,
		"schema_staging":          false,
		"tenant_catalog":          false,
		"client_lifecycle":        false,
		"managed_backups":         false,
		"restore_actions":         false,
	}
	console := ""
	if a != nil && a.Cfg != nil {
		console = strings.TrimSpace(a.Cfg.CORSOrigin)
		if a.Cfg.SchemaVersioningEnabled && !a.Cfg.SchemaVersioningBypass {
			flags["schema_staging"] = true
		}
		if a.Cfg.HostCLICapabilitiesHook != nil {
			for k, v := range a.Cfg.HostCLICapabilitiesHook() {
				flags[k] = v
			}
		}
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"capabilities":   flags,
		"console_url":    console,
		"engine_version": "",
	})
}

func loadDeviceLogin(code, audience string) (*deviceLoginRecord, bool) {
	v, ok := deviceLogins.Load(strings.TrimSpace(code))
	if !ok {
		return nil, false
	}
	rec, ok := v.(*deviceLoginRecord)
	if !ok || rec == nil || rec.Audience != audience {
		return nil, false
	}
	copy := *rec
	if rec.TenantIDs != nil {
		copy.TenantIDs = map[string][]string{}
		for projectID, tenants := range rec.TenantIDs {
			copy.TenantIDs[projectID] = append([]string(nil), tenants...)
		}
	}
	copy.ProjectIDs = append([]string(nil), rec.ProjectIDs...)
	return &copy, true
}

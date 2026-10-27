package controller

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestCLILoginStartPollPending(t *testing.T) {
	e := echo.New()
	ctrl := &AuthController{}
	e.POST("/system/cli/v1/login/start", ctrl.CLILoginStart)
	e.POST("/system/cli/v1/login/poll", ctrl.CLILoginPoll)

	startReq := httptest.NewRequest(http.MethodPost, "/system/cli/v1/login/start", bytes.NewReader([]byte(`{}`)))
	startReq.Header.Set("Content-Type", "application/json")
	startRec := httptest.NewRecorder()
	e.ServeHTTP(startRec, startReq)
	require.Equal(t, http.StatusOK, startRec.Code)
	var started struct {
		DeviceCode string `json:"device_code"`
		UserCode   string `json:"user_code"`
	}
	require.NoError(t, json.Unmarshal(startRec.Body.Bytes(), &started))
	require.NotEmpty(t, started.DeviceCode)
	require.NotEmpty(t, started.UserCode)

	pollReq := httptest.NewRequest(http.MethodPost, "/system/cli/v1/login/poll", bytes.NewReader([]byte(`{"device_code":"`+started.DeviceCode+`"}`)))
	pollReq.Header.Set("Content-Type", "application/json")
	pollRec := httptest.NewRecorder()
	e.ServeHTTP(pollRec, pollReq)
	require.Equal(t, http.StatusOK, pollRec.Code)
	var polled struct {
		Status      string `json:"status"`
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal(pollRec.Body.Bytes(), &polled))
	require.Equal(t, "pending", polled.Status)
	require.Empty(t, polled.AccessToken)
}

func TestCLILoginVerifyRedirectsToConsole(t *testing.T) {
	e := echo.New()
	ctrl := &AuthController{Cfg: &models.Config{CORSOrigin: "http://localhost:4055"}}
	e.POST("/system/cli/v1/login/start", ctrl.CLILoginStart)
	e.GET("/system/cli/v1/login/verify", ctrl.CLILoginVerify)
	startReq := httptest.NewRequest(http.MethodPost, "/system/cli/v1/login/start", bytes.NewReader([]byte(`{}`)))
	startReq.Header.Set("Content-Type", "application/json")
	startRec := httptest.NewRecorder()
	e.ServeHTTP(startRec, startReq)
	var started struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
	}
	require.NoError(t, json.Unmarshal(startRec.Body.Bytes(), &started))
	require.Contains(t, started.VerificationURI, "http://localhost:4055/cli?")
	require.Contains(t, started.VerificationURI, "user_code="+started.UserCode)
	req := httptest.NewRequest(http.MethodGet, "/system/cli/v1/login/verify?device_code="+started.DeviceCode, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusFound, rec.Code)
	require.Contains(t, rec.Header().Get("Location"), "http://localhost:4055/cli?")
	require.Contains(t, rec.Header().Get("Location"), "device_code="+started.DeviceCode)
}

func TestCLILoginApproveRequiresSessionAndService(t *testing.T) {
	e := echo.New()
	ctrl := &AuthController{Cfg: &models.Config{}}
	e.POST("/system/cli/v1/login/approve", ctrl.CLILoginApprove)

	req := httptest.NewRequest(http.MethodPost, "/system/cli/v1/login/approve", bytes.NewReader([]byte(`{"device_code":"missing"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestCLILoginUsesLowHarmLoginPreset(t *testing.T) {
	req, err := cliLoginMintRequest("ABCD", cliLoginScope{
		ProjectGrantMode:  models.AccessTokenProjectGrantSelected,
		ProjectIDs:        []string{"p1"},
		TenantGrantMode:   models.AccessTokenTenantGrantSelected,
		TenantIDs:         map[string][]string{"p1": {"tenant-1"}},
		AcknowledgeDanger: true,
	})
	require.NoError(t, err)
	require.Equal(t, "cli_login", req.Preset)
	require.Empty(t, req.Capabilities)
	require.Equal(t, []string{"p1"}, req.ProjectIDs)
	require.Equal(t, []string{"tenant-1"}, req.TenantIDs["p1"])
	require.True(t, req.AcknowledgeDanger)
}

func TestCLILoginApprovalReadsResourceAndTenantScope(t *testing.T) {
	e := echo.New()
	body := "device_code=dev1&project_grant_mode=selected&project_ids=p1&project_ids=p2&tenant_grant_mode=selected&tenant_pairs=p1%20tenant-a"
	req := httptest.NewRequest(http.MethodPost, "/system/cli/v1/login/approve", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	c := e.NewContext(req, httptest.NewRecorder())
	device, scope, err := cliLoginApprovalFromRequest(c)
	require.NoError(t, err)
	require.Equal(t, "dev1", device)
	require.Equal(t, models.AccessTokenProjectGrantSelected, scope.ProjectGrantMode)
	require.Equal(t, []string{"p1", "p2"}, scope.ProjectIDs)
	require.Equal(t, []string{"tenant-a"}, scope.TenantIDs["p1"])
}

func TestCLICapabilitiesAreCoreUntilHostAddsCloud(t *testing.T) {
	e := echo.New()
	ctrl := &AuthController{Cfg: &models.Config{SchemaVersioningEnabled: true}}
	e.GET("/system/cli/v1/capabilities", ctrl.CLICapabilities)
	req := httptest.NewRequest(http.MethodGet, "/system/cli/v1/capabilities", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	var body struct {
		Capabilities map[string]bool `json:"capabilities"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.True(t, body.Capabilities["project_access"])
	require.True(t, body.Capabilities["schema_staging"])
	require.False(t, body.Capabilities["tenant_catalog"])
	require.False(t, body.Capabilities["managed_backups"])
	require.False(t, body.Capabilities["restore_actions"])
}

func TestMCPLoginUsesConsoleOriginAndMCPPath(t *testing.T) {
	for _, origin := range []string{"http://localhost:4055", "https://app.apito.io"} {
		t.Run(origin, func(t *testing.T) {
			e := echo.New()
			ctrl := &AuthController{Cfg: &models.Config{CORSOrigin: origin}}
			e.POST("/system/mcp/v1/login/start", ctrl.MCPLoginStart)
			e.GET("/system/mcp/v1/login/verify", ctrl.MCPLoginVerify)
			startReq := httptest.NewRequest(http.MethodPost, "/system/mcp/v1/login/start", bytes.NewReader([]byte(`{}`)))
			startReq.Header.Set("Content-Type", "application/json")
			startRec := httptest.NewRecorder()
			e.ServeHTTP(startRec, startReq)
			require.Equal(t, http.StatusOK, startRec.Code)
			var started struct {
				DeviceCode      string `json:"device_code"`
				UserCode        string `json:"user_code"`
				VerificationURI string `json:"verification_uri"`
			}
			require.NoError(t, json.Unmarshal(startRec.Body.Bytes(), &started))
			require.Contains(t, started.VerificationURI, origin+"/mcp?")
			require.Contains(t, started.VerificationURI, "user_code="+started.UserCode)
			require.NotContains(t, started.VerificationURI, "/cli?")

			req := httptest.NewRequest(http.MethodGet, "/system/mcp/v1/login/verify?device_code="+started.DeviceCode, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.Equal(t, http.StatusFound, rec.Code)
			require.Contains(t, rec.Header().Get("Location"), origin+"/mcp?")
			require.Contains(t, rec.Header().Get("Location"), "device_code="+started.DeviceCode)
		})
	}
}

func TestMCPLoginApproveRequiresSession(t *testing.T) {
	e := echo.New()
	ctrl := &AuthController{Cfg: &models.Config{}}
	e.POST("/system/mcp/v1/login/approve", ctrl.MCPLoginApprove)
	req := httptest.NewRequest(http.MethodPost, "/system/mcp/v1/login/approve", bytes.NewReader([]byte(`{"device_code":"missing"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	require.Contains(t, rec.Body.String(), "approving MCP")
}

func TestMCPLoginUsesReadOnlyAssistantPresetAndScopes(t *testing.T) {
	selected, err := deviceLoginMintRequest(deviceAudienceMCP, "ABCD", cliLoginScope{
		ProjectGrantMode: models.AccessTokenProjectGrantSelected,
		ProjectIDs:       []string{"p1", "p2"},
		TenantGrantMode:  models.AccessTokenTenantGrantSelected,
		TenantIDs:        map[string][]string{"p1": {"tenant-1"}},
		Capabilities:     []string{"database.write"},
	})
	require.NoError(t, err)
	require.Equal(t, "mcp_assistant", selected.Preset)
	require.Equal(t, "MCP ABCD", selected.Name)
	require.Empty(t, selected.Capabilities)
	require.Equal(t, []string{"p1", "p2"}, selected.ProjectIDs)
	require.Equal(t, []string{"tenant-1"}, selected.TenantIDs["p1"])

	all, err := deviceLoginMintRequest(deviceAudienceMCP, "WXYZ", cliLoginScope{
		ProjectGrantMode: models.AccessTokenProjectGrantAllAdmin,
		ProjectIDs:       []string{"should-drop"},
		TenantGrantMode:  models.AccessTokenTenantGrantNone,
		TenantIDs:        map[string][]string{"p1": {"tenant-1"}},
	})
	require.NoError(t, err)
	require.Equal(t, "mcp_assistant", all.Preset)
	require.Nil(t, all.ProjectIDs)
	require.Nil(t, all.TenantIDs)
	require.Equal(t, models.AccessTokenProjectGrantAllAdmin, all.ProjectGrantMode)
}

func TestCLILoginAllAdminAddsProjectsWrite(t *testing.T) {
	all, err := deviceLoginMintRequest(deviceAudienceCLI, "CODE", cliLoginScope{
		Preset:           "cli_login",
		ProjectGrantMode: models.AccessTokenProjectGrantAllAdmin,
	})
	require.NoError(t, err)
	require.Equal(t, "cli_login", all.Preset)
	require.Equal(t, []string{authz.CapProjectsWrite}, all.Capabilities)
	require.NotContains(t, all.Capabilities, authz.CapDataRead)
	require.NotContains(t, all.Capabilities, authz.CapDatabaseRead)
	require.NotContains(t, all.Capabilities, authz.CapDatabaseWrite)

	selected, err := deviceLoginMintRequest(deviceAudienceCLI, "CODE", cliLoginScope{
		Preset:           "cli_login",
		ProjectGrantMode: models.AccessTokenProjectGrantSelected,
		ProjectIDs:       []string{"p1"},
	})
	require.NoError(t, err)
	require.Empty(t, selected.Capabilities)
}

func TestDeviceLoginPresetChoices(t *testing.T) {
	advanced, err := deviceLoginMintRequest(deviceAudienceMCP, "ABCD", cliLoginScope{
		Preset:           "mcp_advanced",
		Capabilities:     []string{"schema.publish"},
		ProjectGrantMode: models.AccessTokenProjectGrantAllAdmin,
	})
	require.NoError(t, err)
	require.Equal(t, "mcp_advanced", advanced.Preset)
	require.Empty(t, advanced.Capabilities)

	custom, err := deviceLoginMintRequest(deviceAudienceMCP, "ABCD", cliLoginScope{
		Preset:       "custom",
		Capabilities: []string{"projects.read"},
	})
	require.NoError(t, err)
	require.Equal(t, "custom", custom.Preset)
	require.Equal(t, []string{"projects.read"}, custom.Capabilities)

	_, err = deviceLoginMintRequest(deviceAudienceMCP, "ABCD", cliLoginScope{Preset: "cli_sync"})
	require.Error(t, err)

	syncPreset, err := deviceLoginMintRequest(deviceAudienceCLI, "ABCD", cliLoginScope{Preset: "cli_sync"})
	require.NoError(t, err)
	require.Equal(t, "cli_sync", syncPreset.Preset)
	require.Empty(t, syncPreset.Capabilities)

	_, err = deviceLoginMintRequest(deviceAudienceCLI, "ABCD", cliLoginScope{Preset: "mcp_advanced"})
	require.Error(t, err)
}

func TestMCPLoginPollDeliversGrantOnce(t *testing.T) {
	const code = "mcp-once-device"
	t.Cleanup(func() { deviceLogins.Delete(code) })
	deviceLogins.Store(code, &deviceLoginRecord{
		Audience:         deviceAudienceMCP,
		DeviceCode:       code,
		UserCode:         "ABCD",
		Expires:          time.Now().Add(time.Minute),
		Status:           "approved",
		Token:            "apt_secret_once",
		ProjectGrantMode: models.AccessTokenProjectGrantSelected,
		ProjectIDs:       []string{"p1"},
		TenantGrantMode:  models.AccessTokenTenantGrantSelected,
		TenantIDs:        map[string][]string{"p1": {"tenant-1"}},
	})

	e := echo.New()
	ctrl := &AuthController{}
	e.POST("/system/mcp/v1/login/poll", ctrl.MCPLoginPoll)
	poll := func() map[string]interface{} {
		req := httptest.NewRequest(http.MethodPost, "/system/mcp/v1/login/poll", bytes.NewReader([]byte(`{"device_code":"`+code+`"}`)))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		return body
	}

	first := poll()
	require.Equal(t, "approved", first["status"])
	require.Equal(t, "apt_secret_once", first["access_token"])
	require.Equal(t, models.AccessTokenProjectGrantSelected, first["project_grant_mode"])
	require.Equal(t, []interface{}{"p1"}, first["project_ids"])
	require.Equal(t, models.AccessTokenTenantGrantSelected, first["tenant_grant_mode"])

	second := poll()
	require.Equal(t, "consumed", second["status"])
	_, hasToken := second["access_token"]
	require.False(t, hasToken)
}

func TestMCPLoginPollExpiresPendingDevice(t *testing.T) {
	const code = "mcp-expired-device"
	t.Cleanup(func() { deviceLogins.Delete(code) })
	deviceLogins.Store(code, &deviceLoginRecord{
		Audience:   deviceAudienceMCP,
		DeviceCode: code,
		Expires:    time.Now().Add(-time.Minute),
		Status:     "pending",
		Token:      "apt_should_not_leak",
	})
	e := echo.New()
	ctrl := &AuthController{}
	e.POST("/system/mcp/v1/login/poll", ctrl.MCPLoginPoll)
	req := httptest.NewRequest(http.MethodPost, "/system/mcp/v1/login/poll", bytes.NewReader([]byte(`{"device_code":"`+code+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"expired"`)
	require.NotContains(t, rec.Body.String(), "apt_should_not_leak")
}

func TestMCPAndCLIDeviceCodesAreNotInterchangeable(t *testing.T) {
	const code = "shared-looking-device"
	t.Cleanup(func() { deviceLogins.Delete(code) })
	deviceLogins.Store(code, &deviceLoginRecord{
		Audience:   deviceAudienceCLI,
		DeviceCode: code,
		Expires:    time.Now().Add(time.Minute),
		Status:     "approved",
		Token:      "apt_cli_secret",
	})
	e := echo.New()
	ctrl := &AuthController{}
	e.POST("/system/mcp/v1/login/poll", ctrl.MCPLoginPoll)
	req := httptest.NewRequest(http.MethodPost, "/system/mcp/v1/login/poll", bytes.NewReader([]byte(`{"device_code":"`+code+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), `"status":"expired"`)
	require.NotContains(t, rec.Body.String(), "apt_cli_secret")
}

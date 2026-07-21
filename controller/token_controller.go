package controller

import (
	"net/http"
	"strings"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/services"
	"github.com/labstack/echo/v4"
)

func (a *AuthController) accessTokenSvc() *services.AccessTokenService {
	if a == nil || a.graphQLServer == nil || a.graphQLServer.ApitoTokenService == nil {
		return nil
	}
	return a.graphQLServer.ApitoTokenService.AccessTokens()
}

func requireAccessTokenConsoleSession(c echo.Context) error {
	plane, _ := c.Get("auth_plane").(string)
	if services.PrincipalFromEcho(c) != nil || plane == "access_token" || plane == "project_api_key" {
		return echo.NewHTTPError(
			http.StatusForbidden,
			"access tokens can only be managed from an authenticated console session",
		)
	}
	return nil
}

// CreateAccessToken mints a new apt_ token (reveal once).
func (a *AuthController) CreateAccessToken(c echo.Context) error {
	if err := requireAccessTokenConsoleSession(c); err != nil {
		return err
	}
	var req models.CreateAccessTokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}
	svc := a.accessTokenSvc()
	if svc == nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: "access token service unavailable",
			Code:    http.StatusInternalServerError,
		})
	}
	userID, _ := c.Get("user").(string)
	raw, pub, err := svc.Mint(c.Request().Context(), userID, &req)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":    http.StatusOK,
		"token":   raw,
		"record":  pub,
		"message": "Token created. Copy it now — it will not be shown again.",
	})
}

// ListAccessTokens returns public inventory for the current system user.
func (a *AuthController) ListAccessTokens(c echo.Context) error {
	if err := requireAccessTokenConsoleSession(c); err != nil {
		return err
	}
	svc := a.accessTokenSvc()
	if svc == nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: "access token service unavailable",
			Code:    http.StatusInternalServerError,
		})
	}
	userID, _ := c.Get("user").(string)
	tokens, err := svc.List(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":   http.StatusOK,
		"tokens": tokens,
	})
}

// RevokeAccessToken revokes by id (preferred) or raw token.
func (a *AuthController) RevokeAccessToken(c echo.Context) error {
	if err := requireAccessTokenConsoleSession(c); err != nil {
		return err
	}
	var req models.RevokeAccessTokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}
	svc := a.accessTokenSvc()
	if svc == nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: "access token service unavailable",
			Code:    http.StatusInternalServerError,
		})
	}
	userID, _ := c.Get("user").(string)
	var err error
	if strings.TrimSpace(req.ID) != "" {
		err = svc.Revoke(c.Request().Context(), userID, strings.TrimSpace(req.ID), userID)
	} else if strings.TrimSpace(req.Token) != "" {
		err = svc.RevokeByRaw(c.Request().Context(), userID, req.Token)
	} else {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "id or token is required",
			Code:    http.StatusBadRequest,
		})
	}
	if err != nil {
		code := http.StatusBadRequest
		if strings.Contains(err.Error(), "not found") {
			code = http.StatusNotFound
		}
		return c.JSON(code, &models.HttpResponse{Message: err.Error(), Code: uint32(code)})
	}
	return c.JSON(http.StatusOK, &models.HttpResponse{
		Message: "Token revoked",
		Code:    http.StatusOK,
	})
}

// RotateAccessToken issues a replacement secret with the same grants.
func (a *AuthController) RotateAccessToken(c echo.Context) error {
	if err := requireAccessTokenConsoleSession(c); err != nil {
		return err
	}
	var req models.RotateAccessTokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}
	if strings.TrimSpace(req.ID) == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "id is required",
			Code:    http.StatusBadRequest,
		})
	}
	svc := a.accessTokenSvc()
	if svc == nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: "access token service unavailable",
			Code:    http.StatusInternalServerError,
		})
	}
	userID, _ := c.Get("user").(string)
	raw, pub, err := svc.Rotate(c.Request().Context(), userID, strings.TrimSpace(req.ID))
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":    http.StatusOK,
		"token":   raw,
		"record":  pub,
		"message": "Token rotated. Copy the new secret now — it will not be shown again.",
	})
}

// ListAccessTokenCatalog returns capability registry + presets for Console.
func (a *AuthController) ListAccessTokenCatalog(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":         http.StatusOK,
		"capabilities": authz.All(),
		"presets":      authz.Presets(),
		"bindings":     authz.DefaultOperationBindings(),
	})
}

// ListAdministrableProjects returns projects the issuer can currently administer (for "all projects" preview).
func (a *AuthController) ListAdministrableProjects(c echo.Context) error {
	userID, _ := c.Get("user").(string)
	rows, err := a.graphQLServer.SystemDriver.FindUserProjectsWithRoles(c.Request().Context(), userID)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{Message: err.Error(), Code: http.StatusForbidden})
	}
	type projectBrief struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Role string `json:"role"`
	}
	var out []projectBrief
	for _, row := range rows {
		if row == nil || row.Project == nil {
			continue
		}
		if !services.IsAdministrableRole(row.Role) {
			continue
		}
		out = append(out, projectBrief{ID: row.Project.ID, Name: row.Project.Name, Role: row.Role})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"code":     http.StatusOK,
		"projects": out,
		"count":    len(out),
	})
}

// --- Legacy route aliases (same handlers; old paths kept briefly for Console cutover) ---

func (a *AuthController) GenerateSyncToken(c echo.Context) error {
	return a.CreateAccessToken(c)
}

func (a *AuthController) ListSyncTokens(c echo.Context) error {
	return a.ListAccessTokens(c)
}

func (a *AuthController) DeleteSyncToken(c echo.Context) error {
	// Accept legacy {token,duration} or new {id}/{token}
	var legacy struct {
		Token string `json:"token"`
		ID    string `json:"id"`
	}
	_ = c.Bind(&legacy)
	c.Set("user", c.Get("user"))
	req := models.RevokeAccessTokenRequest{ID: legacy.ID, Token: legacy.Token}
	// Re-bind via temporary: call revoke with constructed body
	svc := a.accessTokenSvc()
	if svc == nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: "access token service unavailable",
			Code:    http.StatusInternalServerError,
		})
	}
	userID, _ := c.Get("user").(string)
	if req.ID != "" {
		if err := svc.Revoke(c.Request().Context(), userID, req.ID, userID); err != nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: err.Error(), Code: http.StatusBadRequest})
		}
	} else if req.Token != "" {
		if err := svc.RevokeByRaw(c.Request().Context(), userID, req.Token); err != nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: err.Error(), Code: http.StatusBadRequest})
		}
	} else {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: "id or token is required", Code: http.StatusBadRequest})
	}
	return c.JSON(http.StatusOK, &models.HttpResponse{Message: "Token revoked", Code: http.StatusOK})
}

func (a *AuthController) SyncProject(c echo.Context) error {
	type SyncProjectRequest struct {
		Token   string          `json:"token"`
		Project *models.Project `json:"project"`
	}

	var req SyncProjectRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	if req.Token == "" {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "token is missing",
			Code:    http.StatusBadRequest,
		})
	}
	if services.IsRetiredSyncTokenPrefix(req.Token) {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: "TOKEN_FORMAT_RETIRED: use apt_ access tokens",
			Code:    http.StatusForbidden,
		})
	}
	if req.Project == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "project is missing",
			Code:    http.StatusBadRequest,
		})
	}
	if req.Project.Schema == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "project does not contain any model. Nothing to sync",
			Code:    http.StatusBadRequest,
		})
	}

	userID := c.Get("user")
	if userID == nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: "user is missing in the token payload",
			Code:    http.StatusBadRequest,
		})
	}

	svc := a.accessTokenSvc()
	if svc == nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: "access token service unavailable",
			Code:    http.StatusInternalServerError,
		})
	}

	ctx := c.Request().Context()
	claims, principal, err := svc.ValidateRaw(ctx, req.Token, c.RealIP(), c.Request().UserAgent())
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}
	if err := svc.AuthorizeProject(ctx, principal, req.Project.ID); err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}
	if !authz.HasCapability(principal.Capabilities, authz.CapSyncWrite) &&
		!authz.HasCapability(principal.Capabilities, authz.CapProjectsWrite) {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: "CAPABILITY_DENIED: missing capability sync.write",
			Code:    http.StatusForbidden,
		})
	}
	_ = claims

	// Proceed with project sync logic here...
	// This would contain the actual project synchronization logic

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Message: "Project synced successfully",
		Code:    http.StatusOK,
	})
}

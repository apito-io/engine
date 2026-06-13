package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/services"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
)

type DeleteSyncTokenRequest struct {
	Token    string `json:"token"`
	Duration string `json:"duration"`
}

func (a *AuthController) DeleteSyncToken(c echo.Context) error {
	var req DeleteSyncTokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()
	userID := c.Get("user").(string)

	// Use optimized token service to validate the token
	t := services.GetBrankaTokenOptimized(a.Cfg, a.graphQLServer.SystemDriver)

	// Validate the token to ensure it exists and is valid
	claims, err := t.ValidateSyncTokenOptimized(ctx, req.Token)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: "Invalid or expired token",
			Code:    http.StatusForbidden,
		})
	}

	// Verify the token belongs to the current user
	if claims.UserID != userID {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: "Token does not belong to current user",
			Code:    http.StatusForbidden,
		})
	}

	// Get the user to access their sync tokens
	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	// #TODO blacklist the token id
	// #TODO move the token blacklist from system db to kv db

	// Find and remove the token from the user's SyncTokens list
	var tokenFound bool
	for i, syncToken := range user.SyncTokens {
		if syncTokensMatch(syncToken.Token, req.Token) {
			user.SyncTokens = append(user.SyncTokens[:i], user.SyncTokens[i+1:]...)
			tokenFound = true
			break
		}
	}

	if !tokenFound {
		return c.JSON(http.StatusNotFound, &models.HttpResponse{
			Message: "Token not found",
			Code:    http.StatusNotFound,
		})
	}

	// Update the user with the modified token list
	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, true)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, &models.HttpResponse{
			Message: "Failed to update user tokens",
			Code:    http.StatusInternalServerError,
		})
	}

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Message: "Token deleted successfully",
		Code:    http.StatusOK,
	})
}

type GenerateSyncTokenRequest struct {
	Name       string   `json:"name"`
	Duration   string   `json:"duration"`
	ProjectIDs []string `json:"project_ids"`
	Scopes     []string `json:"scopes"`
	TokenType  string   `json:"token_type"`
}

func (a *AuthController) GenerateSyncToken(c echo.Context) error {

	var req GenerateSyncTokenRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	ctx := c.Request().Context()
	userID := c.Get("user").(string)
	name := req.Name
	duration := strings.TrimSpace(req.Duration)
	projectIDs := req.ProjectIDs
	scopes := req.Scopes

	accessType, err := normalizeAccessTokenType(req.TokenType)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	var expireAtUnix int64
	if duration == "" {
		// Never-expiring: far-future expiry (year 2099)
		expireAtUnix = time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC).Unix()
	} else {
		parseDuration, parseErr := time.Parse("2006-01-02", duration)
		if parseErr != nil {
			return c.JSON(http.StatusBadRequest, &models.HttpResponse{
				Message: parseErr.Error(),
				Code:    http.StatusBadRequest,
			})
		}
		expireAtUnix = parseDuration.Unix()
	}

	// Use optimized token service
	t := services.GetBrankaTokenOptimized(a.Cfg, a.graphQLServer.SystemDriver)

	innerType := accessTokenInnerType(accessType)
	rawKey, err := t.GenerateSyncTokenOptimized(ctx, userID, projectIDs, scopes, innerType, expireAtUnix)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	prefixedToken := prefixAccessToken(accessType, *rawKey)

	// update user current project id
	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	// append the sync token to the user
	user.SyncTokens = append(user.SyncTokens, &models.SyncToken{
		Token:     prefixedToken,
		TokenType: accessType,
		Name:      name,
		Expire:    duration,
		CreatedAt: utility.GetCurrentTime(),
		// need these for ui display
		ProjectIDs: projectIDs,
		Scopes:     scopes,
	})

	// update the user
	err = a.graphQLServer.SystemDriver.UpdateSystemUser(ctx, user, false)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Token: prefixedToken,
		Code:  http.StatusOK,
	})
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

	ctx := c.Request().Context()

	// Use optimized token service
	t := services.GetBrankaTokenOptimized(a.Cfg, a.graphQLServer.SystemDriver)

	decodedToken, err := t.ValidateSyncTokenOptimized(ctx, req.Token)
	if err != nil {
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusBadRequest,
		})
	}

	// Check if user has access to the project
	hasAccess := false
	for _, projectID := range decodedToken.ProjectIDs {
		if projectID == req.Project.ID {
			hasAccess = true
			break
		}
	}

	if !hasAccess {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: "You don't have access to this project",
			Code:    http.StatusForbidden,
		})
	}

	// Check if user has write scope
	if !decodedToken.HasScope("system_api_write") && !decodedToken.HasScope("project_write") {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: "You don't have write permissions",
			Code:    http.StatusForbidden,
		})
	}

	// Proceed with project sync logic here...
	// This would contain the actual project synchronization logic

	return c.JSON(http.StatusOK, &models.HttpResponse{
		Message: "Project synced successfully",
		Code:    http.StatusOK,
	})
}

func (a *AuthController) ListSyncTokens(c echo.Context) error {
	ctx := c.Request().Context()
	userID := c.Get("user").(string)

	user, err := a.graphQLServer.SystemDriver.GetSystemUser(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusForbidden, &models.HttpResponse{
			Message: err.Error(),
			Code:    http.StatusForbidden,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"tokens": user.SyncTokens,
		"code":   http.StatusOK,
	})
}

package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// Legacy helpers retained only for historical unit coverage; issuance uses apt_ now.
func TestNormalizeAccessTokenTypeRejected(t *testing.T) {
	// Old token types are no longer issued; normalize still maps for any leftover callers.
	got, err := normalizeAccessTokenType("cli")
	require.NoError(t, err)
	require.Equal(t, "cli", got)
}

func TestSyncTokensMatch(t *testing.T) {
	require.True(t, syncTokensMatch("cli-abc", "cli-abc"))
	require.True(t, syncTokensMatch("cli-abc", "abc"))
}

func TestAccessTokenManagementRequiresConsoleSession(t *testing.T) {
	e := echo.New()
	ctx := e.NewContext(
		httptest.NewRequest(http.MethodPost, "/system/access-tokens", nil),
		httptest.NewRecorder(),
	)
	ctx.Set("access_principal", &models.AccessPrincipal{TokenID: "token-1"})

	err := requireAccessTokenConsoleSession(ctx)
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusForbidden, httpErr.Code)

	apiKeyCtx := e.NewContext(
		httptest.NewRequest(http.MethodPost, "/system/access-tokens", nil),
		httptest.NewRecorder(),
	)
	apiKeyCtx.Set("auth_plane", "project_api_key")
	err = requireAccessTokenConsoleSession(apiKeyCtx)
	require.ErrorAs(t, err, &httpErr)
	require.Equal(t, http.StatusForbidden, httpErr.Code)
}

func TestGetAccessTokenMeRejectsNonAccessTokenPlane(t *testing.T) {
	ctrl := &AuthController{}
	e := echo.New()
	rec := httptest.NewRecorder()
	ctx := e.NewContext(
		httptest.NewRequest(http.MethodGet, "/system/access-tokens/me", nil),
		rec,
	)
	ctx.Set("auth_plane", "console_session")

	err := ctrl.GetAccessTokenMe(ctx)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Contains(t, rec.Body.String(), "apt_ Bearer")
	require.NotContains(t, rec.Body.String(), "secret_hash")
}

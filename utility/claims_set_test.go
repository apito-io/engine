package utility

import (
	"net/http/httptest"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestAccessTokenClaimsNeverSelectImplicitProject(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest("POST", "/system/graphql", nil)
	req.Header.Set(models.ApitoProjectIDHeader, "project-b")
	ctx := e.NewContext(req, httptest.NewRecorder())

	err := SetTokenClaimsToRouter(ctx, &models.TokenClaims{
		UserID:     "user-1",
		TokenType:  "access_token",
		ProjectIDs: []string{"project-a", "project-b"},
	})
	require.NoError(t, err)
	require.Nil(t, ctx.Get("project"))
	require.Equal(t, "user-1", ctx.Get("user"))
	require.Equal(t, false, ctx.Get("is_super_admin"))
	got, ok := ctx.Get(EchoTokenClaimsKey).(*models.TokenClaims)
	require.True(t, ok)
	require.False(t, got.IsSuperAdmin)
}

func TestSessionClaimsKeepCanonicalProjectHeaderBehavior(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest("POST", "/system/graphql", nil)
	req.Header.Set(models.ApitoProjectIDHeader, "project-b")
	ctx := e.NewContext(req, httptest.NewRecorder())

	err := SetTokenClaimsToRouter(ctx, &models.TokenClaims{
		UserID:     "user-1",
		ProjectIDs: []string{"project-a", "project-b"},
	})
	require.NoError(t, err)
	require.Equal(t, "project-b", ctx.Get("project"))
}

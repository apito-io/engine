package services

import (
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func bootstrapContext(role string) echo.Context {
	e := echo.New()
	ctx := e.NewContext(httptest.NewRequest("POST", "/system/graphql", nil), httptest.NewRecorder())
	ctx.Set("auth_plane", "project_api_key")
	ctx.Set("role", role)
	return ctx
}

func TestBootstrapAllowlistAcceptsAuthOperations(t *testing.T) {
	ctx := bootstrapContext("login")
	for _, query := range []string{
		`query { loginUser(phone: "1", secret: "x") { token } }`,
		`mutation { registerUser(phone: "1") { id } }`,
		`query { googleOAuthState { state } }`,
		`query { oauthState { state } }`,
		`query { searchTenantsByDomain(domain: "a.example") { id } }`,
		`query { clientRuntime(platform: "android") { platform force_update maintenance extras } }`,
	} {
		require.NoError(t, EnforceBootstrapSystemAllowlist(ctx, query), query)
	}
}

func TestBootstrapAllowlistRejectsPrivilegedAndMixedDocuments(t *testing.T) {
	ctx := bootstrapContext("passenger")
	for _, query := range []string{
		``,
		`mutation { createTenant(name: "x") { id } }`,
		`query { getTenants { id } }`,
		`query { searchTenants(project_id: "p") { id } }`,
		`mutation { generateTenantToken(tenant_id: "t", duration: "2026-01-01") { token } }`,
		`query { getModelData(model: "post") { id } }`,
		`query { getModelData(model: "app_release_policy") { id } }`,
		`query { getModelData(model: "app_config") { id } }`,
		`query { clientRuntime(platform: "android") { platform } getModelData(model: "app_release_policy") { id } }`,
		`query { alias: getModelData(model: "app_release_policy") { id } }`,
		`query { ...Frag } fragment Frag on Query { getModelData(model: "app_config") { id } }`,
		`query { loginUser(phone: "1", secret: "x") { token } createTenant(name: "x") { id } }`,
		`query { alias: createTenant(name: "x") { id } }`,
		`query { ...Frag } fragment Frag on Query { deleteTenant(tenant_id: "t") }`,
		`query { ...Frag } fragment Frag on Query { loginUser(phone: "1", secret: "x") { token } }`,
		`query { ... on Query { loginUser(phone: "1", secret: "x") { token } } }`,
		`query { __schema { types { name } } }`,
		`query { loginUser(phone: "1", secret: "x") { alias: token } }`,
		`query { loginUser(phone: "1", secret: "x") { ... on LoginUserPayload { token } } }`,
		`query { loginUser(phone: "1", secret: "x") @skip(if: true) { token } }`,
		`query @skip(if: true) { loginUser(phone: "1", secret: "x") { token } }`,
		`query { loginUser(phone: "1", secret: "x") { user { __typename } } }`,
	} {
		err := EnforceBootstrapSystemAllowlist(ctx, query)
		require.Error(t, err, query)
	}
	require.Error(t, EnforceBootstrapSystemAllowlist(ctx, `query { alias: loginUser(phone: "1", secret: "x") { token } }`))
	require.Error(t, EnforceBootstrapSystemAllowlist(ctx, `query First { loginUser(phone: "1", secret: "x") { token } } query Second { loginUser(phone: "1", secret: "x") { token } }`))
	require.Error(t, EnforceBootstrapSystemAllowlist(ctx, `query { ...A } fragment A on Query { ...B } fragment B on Query { ...A }`))
	require.Error(t, EnforceBootstrapSystemAllowlist(ctx, `query { ...A } fragment A on Query { ...B } fragment B on Query { ...C } fragment C on Query { loginUser(phone: "1", secret: "x") { token } }`))
}

func TestBootstrapAllowlistAcceptsNestedSelectionWithoutBootstrapSyntax(t *testing.T) {
	ctx := bootstrapContext("login")
	require.NoError(t, EnforceBootstrapSystemAllowlist(ctx,
		`query { loginUser(phone: "1", secret: "x") { token user { id role } } }`))
}

func TestBootstrapAllowlistRequiresCorrectOperationKind(t *testing.T) {
	ctx := bootstrapContext("login")
	require.Error(t, EnforceBootstrapSystemAllowlist(ctx, `mutation { loginUser(phone: "1", secret: "x") { token } }`))
	require.Error(t, EnforceBootstrapSystemAllowlist(ctx, `mutation { googleOAuthState { state } }`))
}

func TestBootstrapAllowlistDeniesProjectUserData(t *testing.T) {
	ctx := bootstrapContext("smokerw")
	ctx.Set("is_project_user", true)
	require.Error(t, EnforceBootstrapSystemAllowlist(ctx, `query { getModelData(model: "note") { id } }`))
	require.NoError(t, EnforceBootstrapSystemAllowlist(ctx, `query { loginUser(phone: "1", secret: "x") { token } }`))
}

func TestBootstrapAllowlistSkipsAdminAndOperatorPlanes(t *testing.T) {
	admin := bootstrapContext("admin")
	require.NoError(t, EnforceBootstrapSystemAllowlist(admin, `mutation { deleteTenant(tenant_id: "t") }`))
	appUserAdmin := bootstrapContext("admin")
	appUserAdmin.Set("is_project_user", true)
	require.Error(t, EnforceBootstrapSystemAllowlist(appUserAdmin, `mutation { deleteTenant(tenant_id: "t") }`))
	e := echo.New()
	operator := e.NewContext(httptest.NewRequest("POST", "/system/graphql", nil), httptest.NewRecorder())
	operator.Set("auth_plane", "access_token")
	require.NoError(t, EnforceBootstrapSystemAllowlist(operator, `mutation { createTenant(name: "x") { id } }`))
}

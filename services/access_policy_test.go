package services

import (
	"net/http/httptest"
	"testing"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestRequireCapabilityEnforce(t *testing.T) {
	e := echo.New()
	req := e.NewContext(nil, nil)
	// no principal → allow
	require.NoError(t, RequireCapability(req, authz.CapDataRead))

	principal := &models.AccessPrincipal{
		TokenID:      "t1",
		IssuerUserID: "u1",
		Capabilities: []string{authz.CapDataRead},
	}
	// Need a real echo context with request for audit path
	_ = principal
}

func TestCapabilityDeniedError(t *testing.T) {
	err := &CapabilityDeniedError{Capability: authz.CapDataWrite}
	require.Contains(t, err.Error(), "CAPABILITY_DENIED")
	require.Contains(t, err.Error(), authz.CapDataWrite)
}

func TestRequireDataGraphQLCapabilities(t *testing.T) {
	t.Setenv("APITO_ACCESS_POLICY_MODE", AccessPolicyEnforce)
	e := echo.New()
	newContext := func(capabilities ...string) echo.Context {
		ctx := e.NewContext(
			httptest.NewRequest("POST", "/secured/graphql", nil),
			httptest.NewRecorder(),
		)
		ctx.Set("access_principal", &models.AccessPrincipal{
			TokenID:      "t1",
			IssuerUserID: "u1",
			Capabilities: capabilities,
		})
		return ctx
	}

	require.NoError(t, RequireDataGraphQLCapabilities(
		newContext(authz.CapDataRead),
		`query { listOfFood { id } }`,
	))
	require.NoError(t, RequireDataGraphQLCapabilities(
		newContext(authz.CapDataWrite),
		`mutation { createFood(input: {}) { id } }`,
	))
	require.NoError(t, RequireDataGraphQLCapabilities(
		newContext(authz.CapDataDelete),
		`mutation { deleteFood(id: "1") }`,
	))
	require.Error(t, RequireDataGraphQLCapabilities(
		newContext(authz.CapDataWrite),
		`mutation { deleteFood(id: "1") }`,
	))
	require.NoError(t, RequireDataGraphQLCapabilities(
		newContext(authz.CapRelationsWrite),
		`mutation { connectFoodToCategory(from: "1", to: "2") }`,
	))
}

func TestRequireSystemGraphQLCapabilities(t *testing.T) {
	t.Setenv("APITO_ACCESS_POLICY_MODE", AccessPolicyEnforce)
	e := echo.New()
	newContext := func(capabilities ...string) echo.Context {
		ctx := e.NewContext(
			httptest.NewRequest("POST", "/system/graphql", nil),
			httptest.NewRecorder(),
		)
		ctx.Set("access_principal", &models.AccessPrincipal{
			TokenID:      "t1",
			IssuerUserID: "u1",
			Capabilities: capabilities,
		})
		return ctx
	}

	require.NoError(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapProjectsRead),
		`query { currentProject { id } }`,
	))
	require.Error(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapProjectsRead),
		`mutation { upsertRoleToProject(input: {}) { id } }`,
	))
	require.NoError(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapRolesWrite),
		`mutation { upsertRoleToProject(input: {}) { id } }`,
	))
	require.NoError(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapProjectsWrite),
		`mutation { generateProjectToken(name: "x") { token } }`,
	))
	require.NoError(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapRolesRead),
		`query { listPermissionsAndScopes }`,
	))
	// no principal → allow
	require.NoError(t, RequireSystemGraphQLCapabilities(
		e.NewContext(httptest.NewRequest("POST", "/system/graphql", nil), httptest.NewRecorder()),
		`query { currentProject { id } }`,
	))
}

func TestRequireSecuredRESTCapability(t *testing.T) {
	t.Setenv("APITO_ACCESS_POLICY_MODE", AccessPolicyEnforce)
	e := echo.New()
	newContext := func(capabilities ...string) echo.Context {
		ctx := e.NewContext(
			httptest.NewRequest("POST", "/secured/rest/food", nil),
			httptest.NewRecorder(),
		)
		ctx.Set("access_principal", &models.AccessPrincipal{
			TokenID:      "t1",
			IssuerUserID: "u1",
			Capabilities: capabilities,
		})
		return ctx
	}

	require.NoError(t, RequireSecuredRESTCapability(
		newContext(authz.CapDataRead), "/secured/rest/food", "GET",
	))
	require.Error(t, RequireSecuredRESTCapability(
		newContext(authz.CapDataRead), "/secured/rest/food", "POST",
	))
	require.NoError(t, RequireSecuredRESTCapability(
		newContext(authz.CapFilesDelete), "/secured/files/media/1", "DELETE",
	))
	require.Error(t, RequireSecuredRESTCapability(
		newContext(authz.CapFilesWrite), "/secured/files/media/1", "DELETE",
	))
}

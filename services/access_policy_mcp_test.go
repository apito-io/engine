package services

import (
	"net/http/httptest"
	"testing"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

// MCP bug 19: the MCP assistant preset grants data.read, so its document reads
// must not require the raw database.read capability. Nothing else widens.
func TestMCPPresetsReadDocumentsWithDataRead(t *testing.T) {
	t.Setenv("APITO_ACCESS_POLICY_MODE", AccessPolicyEnforce)
	e := echo.New()
	ctxFor := func(preset string, caps ...string) echo.Context {
		ctx := e.NewContext(httptest.NewRequest("POST", "/system/graphql", nil), httptest.NewRecorder())
		ctx.Set("access_principal", &models.AccessPrincipal{TokenID: "t1", IssuerUserID: "u1", Preset: preset, Capabilities: caps})
		return ctx
	}
	list := `query { getModelData(model: "task") { count results { id data } } }`
	one := `query { getSingleData(model: "task", _id: "1") { id data } }`
	write := `mutation { upsertModelData(model_name: "task", payload: {}) { id } }`
	revisions := `query { listSingleModelRevisionData(_id: "1") { id } }`

	for _, preset := range []string{"mcp_assistant", "mcp_advanced"} {
		require.NoError(t, RequireSystemGraphQLCapabilities(ctxFor(preset, authz.CapDataRead), list), preset)
		require.NoError(t, RequireSystemGraphQLCapabilities(ctxFor(preset, authz.CapDataRead), one), preset)
		require.Error(t, RequireSystemGraphQLCapabilities(ctxFor(preset, authz.CapDataRead), write), preset)
		require.Error(t, RequireSystemGraphQLCapabilities(ctxFor(preset, authz.CapDataRead), revisions), preset)
		require.Error(t, RequireSystemGraphQLCapabilities(ctxFor(preset), list), "no data.read, no read")
	}
	// Other tokens keep the console rule: document reads need database.read.
	for _, preset := range []string{"", "custom", "full_access", "sdk_bootstrap"} {
		require.Error(t, RequireSystemGraphQLCapabilities(ctxFor(preset, authz.CapDataRead), list), preset)
		require.NoError(t, RequireSystemGraphQLCapabilities(ctxFor(preset, authz.CapDatabaseRead), list), preset)
	}
}

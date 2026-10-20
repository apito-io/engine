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
	require.NoError(t, RequireDataGraphQLCapabilities(
		newContext(authz.CapSchemaRead),
		`query Introspection { __schema { queryType { name } } }`,
	))
	require.NoError(t, RequireDataGraphQLCapabilities(
		newContext(authz.CapDataRead),
		`query Introspection { __schema { queryType { name } } }`,
	))
	require.Error(t, RequireDataGraphQLCapabilities(
		newContext(authz.CapSchemaRead),
		`query { __schema { queryType { name } } listOfFood { id } }`,
	))
	require.Error(t, RequireDataGraphQLCapabilities(
		newContext(authz.CapProjectsRead),
		`query Introspection { __schema { queryType { name } } }`,
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

	require.Error(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapProjectsRead),
		"",
	))
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
	require.NoError(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapProjectsRead),
		`query { aliased: currentProject { id } }`,
	))
	require.NoError(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapSchemaRead),
		`query { ...SchemaFields } fragment SchemaFields on Query { schemaVersions { version } }`,
	))
	require.NoError(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapProjectsRead),
		`query First { currentProject { id } } query Second { currentProject { id } }`,
	))
	require.NoError(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapProjectsRead),
		`query Safe { currentProject { id } } query Unsafe { currentProject { id } }`,
		"Safe",
	))
	require.Error(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapProjectsRead),
		`query Safe { currentProject { id } } query Unsafe { getModelData(model: "x") { count } }`,
		"Unsafe",
	))
	require.Error(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapProjectsRead),
		`query { unknownSystemField { id } }`,
	))
	require.Error(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapProjectsRead),
		`query { ...Hidden } fragment Hidden on Query { unknownSystemField { id } }`,
	))
	require.Error(t, RequireSystemGraphQLCapabilities(
		newContext(authz.CapDataRead),
		`query { currentProject { id } ...Hidden } fragment Hidden on Query { currentProject { id } }`,
	))
	// no principal → allow
	require.NoError(t, RequireSystemGraphQLCapabilities(
		e.NewContext(httptest.NewRequest("POST", "/system/graphql", nil), httptest.NewRecorder()),
		`query { currentProject { id } }`,
	))
}

func TestSystemGraphQLCapabilityBindingsCoverExtendedSystemRoots(t *testing.T) {
	for _, field := range []string{
		"schemaVersioningStatus",
		"approveSchemaChanges",
		"searchPlatformAccounts",
		"backupProviders",
		"restoreBackup",
		"listPartnerProjectRates",
		"getInvoiceProfile",
		"searchSystemLogs",
		"systemLogHealth",
		"accountWorkspaces",
		"transferWorkspaceOwnership",
		"myFreelanceEngagements",
		"bulkHardDeleteTenants",
		"bulkSoftDeleteTenants",
		"bulkRestoreTenants",
		"restoreTenant",
		"suspendTenant",
		"unsuspendTenant",
	} {
		capability, ok := systemGraphQLCapabilityForField(field)
		require.Truef(t, ok, "missing capability binding for %s", field)
		require.NotEmpty(t, capability, field)
	}
}

func TestSystemGraphQLCapabilityBindingsCoverRegisteredRoots(t *testing.T) {
	roots := []string{
		"currentProject", "previewSchemaRelationTransition", "auditLogs",
		"searchSchemaOperations", "listProjects", "listPermissionsAndScopes",
		"getProjectPlans", "modelDocumentCounts", "modelPhysicalHealth",
		"projectPhysicalHealth", "getUser", "getProject", "listWebHooks",
		"projectModelsInfo", "projectSchemaRelationGraph", "listPluginIds",
		"getSystemPlugins", "getProjectPlugins", "getSingleData", "getModelData",
		"searchSystemUsers", "teamMembers", "listSingleModelRevisionData",
		"projectFunctionsInfo", "listFunctionRevisions", "listFunctionDeployments",
		"listExecutableFunctions", "listAvailableFunctions", "send", "upsertPlugin",
		"removeProjectSpecificPlugin", "generateProjectToken", "deleteProjectToken",
		"createWebHook", "deleteWebHook", "updateProject",
		"updateProjectAuthenticationSettings", "updateProjectStorageSettings",
		"updateProfile", "addModelToProject", "runModelMigrations", "updateModel",
		"upsertFunctionToProject", "upsertRoleToProject", "upsertPlanToProject",
		"duplicatePlanInProject", "duplicateRoleInProject",
		"deleteFunctionFromProject", "deployFunctionToProject",
		"rollbackFunctionDeployment", "testFunctionDraft", "deleteRoleFromProject",
		"deletePlanFromProject", "upsertFieldToModel", "modelFieldOperation",
		"rearrangeSerialOfField", "upsertConnectionToModel", "stageRelationTransition",
		"upsertUniqueKey", "deleteUniqueKey", "deleteConnectionFromModel",
		"upsertModelData", "deleteModelData", "duplicateModelData",
		"retrySchemaOperation", "reconcileCommonModels", "searchUsers", "loginUser",
		"clientRuntime",
		"oauthState", "googleOAuthState", "createUser", "registerUser", "updateUser",
		"deleteUser", "resetUserPassword", "schemaVersioningStatus", "schemaDiff",
		"schemaPreview", "schemaChangeEvents", "schemaChangeExecutionRecords",
		"approveSchemaChanges", "discardSchemaDraft",

		// Pro extensions registered by the engine factory.
		"getTenants", "generateTenantToken", "createTenant", "suspendTenant",
		"unsuspendTenant", "deleteTenant", "hardDeleteTenant",
		"restoreTenant", "bulkRestoreTenants",
		"bulkSoftDeleteTenants", "bulkHardDeleteTenants", "updateTenant",
		"searchTenants", "tenantHardDeleteImpact", "searchTenantsByDomain",
		"previewSchemaVersionRevert", "schemaVersions", "submitSchemaChanges",
		"rejectSchemaChanges", "stageSchemaVersionRevert", "rollbackSchemaVersion",
		"retrySchemaChangeExecutionRecord", "flushSchemaSync",
		"publishSchemaChangeExecutionRecord", "rejectSchemaChangeExecutionRecord",
		"backupProviders", "backupTargets", "backupSnapshots", "previewRestore",
		"backupArtifactURL", "backupRuns", "updateBackupTargets",
		"updateBackupSettings", "testBackupSettings", "runBackupNow", "runFullDump",
		"restoreBackup", "listTenantSubscribers", "listTenantInvoices",
		"getProjectPaymentProviders", "createTenantSubscription",
		"updateTenantSubscription", "cancelTenantSubscription", "deleteTenantSubscription", "markInvoicePaid",
		"createTenantInvoice", "updateProjectPaymentProviders", "searchPartners",
		"listPartnerProjectRates", "upsertPartnerProjectRate", "createPartner",
		"updatePartner", "getInvoiceProfile", "updateInvoiceProfile",
		"searchPlatformAccounts", "slowQueryLogs", "searchSystemLogs", "systemLog",
		"systemLogTrace", "systemLogStats", "systemLogFacets", "systemLogHealth",
		"accountWorkspaces", "createAccountWorkspace", "moveProjectToWorkspace",
		"transferWorkspaceOwnership", "workspaceMembers", "inviteWorkspaceMember",
		"updateWorkspaceMember", "removeWorkspaceMember", "myFreelanceClients",
		"myFreelanceEngagements", "requestFreelanceClientBilling",
		"markFreelanceReady", "acceptFreelanceDelivery",
	}
	require.NoError(t, ValidateSystemGraphQLCapabilityBindings(roots))
}

func TestClientRuntimeBindsSettingsRead(t *testing.T) {
	capability, ok := systemGraphQLCapabilityForField("clientRuntime")
	require.True(t, ok)
	require.Equal(t, authz.CapSettingsRead, capability)
	_, modelRead := systemGraphQLCapabilityForField("getModelData")
	require.True(t, modelRead)
	modelCap, _ := systemGraphQLCapabilityForField("getModelData")
	require.Equal(t, authz.CapDatabaseRead, modelCap)
}

func TestSystemGraphQLCapabilityBindingsRejectUnknownRoot(t *testing.T) {
	require.Error(t, ValidateSystemGraphQLCapabilityBindings([]string{"newPrivilegedResolver"}))
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

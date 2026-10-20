package services

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

// AccessPolicyMode controls capability enforcement.
const (
	AccessPolicyReportOnly = "report_only"
	AccessPolicyEnforce    = "enforce"
)

// AccessPolicyModeFromEnv reads APITO_ACCESS_POLICY_MODE (default enforce for apt_ tokens).
func AccessPolicyModeFromEnv() string {
	m := strings.ToLower(strings.TrimSpace(os.Getenv("APITO_ACCESS_POLICY_MODE")))
	switch m {
	case AccessPolicyReportOnly, "report-only", "report":
		return AccessPolicyReportOnly
	case "", AccessPolicyEnforce:
		return AccessPolicyEnforce
	default:
		return AccessPolicyEnforce
	}
}

// PrincipalFromEcho returns AccessPrincipal if set by apt_ middleware.
func PrincipalFromEcho(ctx echo.Context) *models.AccessPrincipal {
	if ctx == nil {
		return nil
	}
	if v := ctx.Get("access_principal"); v != nil {
		if p, ok := v.(*models.AccessPrincipal); ok {
			return p
		}
	}
	return nil
}

// RequireCapability checks capability for apt_ principals.
// Cookie/JWT console sessions and ak_ keys skip this gate (different auth planes).
// Returns CAPABILITY_DENIED structured message when enforcing.
func RequireCapability(ctx echo.Context, capability string) error {
	principal := PrincipalFromEcho(ctx)
	if principal == nil {
		return nil
	}
	capability = strings.TrimSpace(capability)
	if capability == "" {
		return nil
	}
	ok := authz.HasCapability(principal.Capabilities, capability)
	mode := AccessPolicyModeFromEnv()
	if ok {
		auditAccessDecision(ctx, principal, capability, "allow")
		return nil
	}
	auditAccessDecision(ctx, principal, capability, "deny")
	if mode == AccessPolicyReportOnly {
		log.Printf("[access-policy:report-only] deny token=%s issuer=%s capability=%s path=%s",
			principal.TokenID, principal.IssuerUserID, capability, safePath(ctx))
		return nil
	}
	return &CapabilityDeniedError{Capability: capability}
}

// RequireDataGraphQLCapabilities maps secured project GraphQL operations to
// apt_ data capabilities. Non-apt auth planes remain governed by their own
// role/API-key checks.
func RequireDataGraphQLCapabilities(ctx echo.Context, query string) error {
	if PrincipalFromEcho(ctx) == nil || strings.TrimSpace(query) == "" {
		return nil
	}
	doc, err := parser.ParseQuery(&ast.Source{Input: query})
	if err != nil {
		return err
	}
	if len(doc.Operations) != 1 {
		return fmt.Errorf("exactly one GraphQL operation is required")
	}
	op := doc.Operations[0]
	if op.Operation != ast.Mutation && introspectionOnly(op) {
		if hasSchemaOrDataRead(ctx) {
			return nil
		}
		return RequireCapability(ctx, authz.CapSchemaRead)
	}
	if op.Operation != ast.Mutation {
		return RequireCapability(ctx, authz.CapDataRead)
	}
	required := authz.CapDataWrite
	for _, selection := range op.SelectionSet {
		field, ok := selection.(*ast.Field)
		if !ok {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(field.Name))
		switch {
		case strings.Contains(name, "delete"), strings.Contains(name, "remove"):
			required = authz.CapDataDelete
		case strings.Contains(name, "connect"), strings.Contains(name, "disconnect"), strings.Contains(name, "relation"):
			if required != authz.CapDataDelete {
				required = authz.CapRelationsWrite
			}
		}
	}
	return RequireCapability(ctx, required)
}

// introspectionOnly is a published-schema read. cli_login has schema.read and
// must not receive data.read just to generate types.
func introspectionOnly(op *ast.OperationDefinition) bool {
	if op == nil || len(op.SelectionSet) == 0 {
		return false
	}
	for _, selection := range op.SelectionSet {
		field, ok := selection.(*ast.Field)
		if !ok {
			return false
		}
		if field.Name != "__schema" && field.Name != "__type" {
			return false
		}
	}
	return true
}

func hasSchemaOrDataRead(ctx echo.Context) bool {
	principal := PrincipalFromEcho(ctx)
	if principal == nil {
		return true
	}
	return authz.HasCapability(principal.Capabilities, authz.CapSchemaRead) ||
		authz.HasCapability(principal.Capabilities, authz.CapDataRead)
}

// systemGraphQLFieldAliases contains explicit bindings for system roots that
// predate the capability registry or use a resolver-specific name. Keeping
// this map exhaustive prevents a capability-bearing apt_ token from being
// treated as an anonymous "known" operation while preserving the existing
// operator surface.
var systemGraphQLFieldAliases = map[string]string{
	"previewSchemaRelationTransition":     authz.CapSchemaRead,
	"auditLogs":                           authz.CapAuditRead,
	"searchSchemaOperations":              authz.CapSchemaRead,
	"modelDocumentCounts":                 authz.CapDatabaseRead,
	"modelPhysicalHealth":                 authz.CapDatabaseRead,
	"projectPhysicalHealth":               authz.CapDatabaseRead,
	"getUser":                             authz.CapMembersRead,
	"getProject":                          authz.CapProjectsRead,
	"listWebHooks":                        authz.CapSettingsRead,
	"projectModelsInfo":                   authz.CapSchemaRead,
	"projectSchemaRelationGraph":          authz.CapSchemaRead,
	"listPluginIds":                       authz.CapPluginsRead,
	"getSystemPlugins":                    authz.CapPluginsRead,
	"getProjectPlugins":                   authz.CapPluginsRead,
	"getSingleData":                       authz.CapDatabaseRead,
	"getModelData":                        authz.CapDatabaseRead,
	"searchSystemUsers":                   authz.CapMembersRead,
	"teamMembers":                         authz.CapMembersRead,
	"listSingleModelRevisionData":         authz.CapDatabaseRead,
	"projectFunctionsInfo":                authz.CapFunctionsRead,
	"listFunctionRevisions":               authz.CapFunctionsRead,
	"listFunctionDeployments":             authz.CapFunctionsRead,
	"listExecutableFunctions":             authz.CapFunctionsRead,
	"thirdPartyFunctionQuery":             authz.CapFunctionsRead,
	"listAvailableFunctions":              authz.CapFunctionsRead,
	"send":                                authz.CapSettingsWrite,
	"upsertPlugin":                        authz.CapPluginsWrite,
	"removeProjectSpecificPlugin":         authz.CapPluginsWrite,
	"createWebHook":                       authz.CapSettingsWrite,
	"deleteWebHook":                       authz.CapSettingsWrite,
	"updateProjectAuthenticationSettings": authz.CapSettingsWrite,
	"updateProjectStorageSettings":        authz.CapSettingsWrite,
	"updateProfile":                       authz.CapMembersWrite,
	"addModelToProject":                   authz.CapSchemaWrite,
	"runModelMigrations":                  authz.CapSchemaWrite,
	"updateModel":                         authz.CapSchemaWrite,
	"upsertFunctionToProject":             authz.CapFunctionsWrite,
	"deleteFunctionFromProject":           authz.CapFunctionsDelete,
	"deployFunctionToProject":             authz.CapFunctionsDeploy,
	"rollbackFunctionDeployment":          authz.CapFunctionsDeploy,
	"testFunctionDraft":                   authz.CapFunctionsTest,
	"upsertFieldToModel":                  authz.CapSchemaWrite,
	"modelFieldOperation":                 authz.CapSchemaWrite,
	"rearrangeSerialOfField":              authz.CapSchemaWrite,
	"upsertConnectionToModel":             authz.CapSchemaWrite,
	"stageRelationTransition":             authz.CapSchemaWrite,
	"upsertUniqueKey":                     authz.CapSchemaWrite,
	"deleteUniqueKey":                     authz.CapSchemaWrite,
	"deleteConnectionFromModel":           authz.CapSchemaWrite,
	"upsertModelData":                     authz.CapDatabaseWrite,
	"deleteModelData":                     authz.CapDatabaseWrite,
	"duplicateModelData":                  authz.CapDatabaseWrite,
	"retrySchemaOperation":                authz.CapSchemaWrite,
	"reconcileCommonModels":               authz.CapSchemaWrite,
	"searchUsers":                         authz.CapMembersRead,
	"createUser":                          authz.CapMembersWrite,
	"updateUser":                          authz.CapMembersWrite,
	"deleteUser":                          authz.CapMembersWrite,
	"resetUserPassword":                   authz.CapMembersWrite,
	"oauthState":                          authz.CapAuthLogin,
	"googleOAuthState":                    authz.CapAuthLogin,
	"schemaVersioningStatus":              authz.CapSchemaRead,
	"schemaDiff":                          authz.CapSchemaRead,
	"schemaPreview":                       authz.CapSchemaRead,
	"schemaChangeEvents":                  authz.CapSchemaRead,
	"schemaChangeExecutionRecords":        authz.CapSchemaRead,
	"approveSchemaChanges":                authz.CapSchemaPublish,
	"discardSchemaDraft":                  authz.CapSchemaWrite,
	"upsertRoleToProject":                 authz.CapRolesWrite,
	"deleteRoleFromProject":               authz.CapRolesWrite,
	"duplicateRoleInProject":              authz.CapRolesWrite,
	"deleteRole":                          authz.CapRolesWrite,
	"getProjectPlans":                     authz.CapPlansRead,
	"upsertPlanToProject":                 authz.CapPlansWrite,
	"duplicatePlanInProject":              authz.CapPlansWrite,
	"deletePlanFromProject":               authz.CapPlansWrite,
	"generateProjectToken":                authz.CapProjectsWrite,
	"deleteProjectToken":                  authz.CapProjectsWrite,
	"currentProject":                      authz.CapProjectsRead,
	"getProjectRoles":                     authz.CapRolesRead,
	"listPermissionsAndScopes":            authz.CapRolesRead,
	"generateTenantToken":                 authz.CapTenantsWrite,
	"bulkHardDeleteTenants":               authz.CapTenantsDelete,
	"bulkSoftDeleteTenants":               authz.CapTenantsDelete,
	"bulkRestoreTenants":                  authz.CapTenantsWrite,
	"suspendTenant":                       authz.CapTenantsWrite,
	"unsuspendTenant":                     authz.CapTenantsWrite,
	"tenantHardDeleteImpact":              authz.CapTenantsDelete,
	"listTenantSubscribers":               authz.CapTenantsRead,
	"listTenantInvoices":                  authz.CapTenantsRead,
	"getProjectPaymentProviders":          authz.CapPlansRead,
	"updateProjectPaymentProviders":       authz.CapPlansWrite,
	"createTenantSubscription":            authz.CapTenantsWrite,
	"updateTenantSubscription":            authz.CapTenantsWrite,
	"cancelTenantSubscription":            authz.CapTenantsWrite,
	"deleteTenantSubscription":            authz.CapTenantsWrite,
	"createTenantInvoice":                 authz.CapTenantsWrite,
	"markInvoicePaid":                     authz.CapTenantsWrite,
	"searchPlatformAccounts":              authz.CapMembersRead,
	"previewSchemaVersionRevert":          authz.CapSchemaRead,
	"schemaVersions":                      authz.CapSchemaRead,
	"submitSchemaChanges":                 authz.CapSchemaWrite,
	"rejectSchemaChanges":                 authz.CapSchemaWrite,
	"stageSchemaVersionRevert":            authz.CapSchemaWrite,
	"rollbackSchemaVersion":               authz.CapSchemaPublish,
	"retrySchemaChangeExecutionRecord":    authz.CapSchemaWrite,
	"flushSchemaSync":                     authz.CapSchemaWrite,
	"publishSchemaChangeExecutionRecord":  authz.CapSchemaPublish,
	"rejectSchemaChangeExecutionRecord":   authz.CapSchemaWrite,
	"backupProviders":                     authz.CapSettingsRead,
	"backupTargets":                       authz.CapSettingsRead,
	"backupSnapshots":                     authz.CapSettingsRead,
	"previewRestore":                      authz.CapSettingsRead,
	"backupArtifactURL":                   authz.CapSettingsRead,
	"backupRuns":                          authz.CapSettingsRead,
	"updateBackupTargets":                 authz.CapSettingsWrite,
	"updateBackupSettings":                authz.CapSettingsWrite,
	"testBackupSettings":                  authz.CapSettingsWrite,
	"runBackupNow":                        authz.CapSettingsWrite,
	"runFullDump":                         authz.CapSettingsWrite,
	"restoreBackup":                       authz.CapSettingsWrite,
	"listPartnerProjectRates":             authz.CapTenantsRead,
	"upsertPartnerProjectRate":            authz.CapTenantsWrite,
	"getInvoiceProfile":                   authz.CapPlansRead,
	"updateInvoiceProfile":                authz.CapPlansWrite,
	"slowQueryLogs":                       authz.CapAuditRead,
	"searchSystemLogs":                    authz.CapAuditRead,
	"systemLog":                           authz.CapAuditRead,
	"systemLogTrace":                      authz.CapAuditRead,
	"systemLogStats":                      authz.CapAuditRead,
	"systemLogFacets":                     authz.CapAuditRead,
	"systemLogHealth":                     authz.CapAuditRead,
	"accountWorkspaces":                   authz.CapProjectsRead,
	"createAccountWorkspace":              authz.CapProjectsWrite,
	"moveProjectToWorkspace":              authz.CapProjectsWrite,
	"transferWorkspaceOwnership":          authz.CapProjectsWrite,
	"myFreelanceClients":                  authz.CapTenantsRead,
	"myFreelanceEngagements":              authz.CapTenantsRead,
	"requestFreelanceClientBilling":       authz.CapTenantsWrite,
	"markFreelanceReady":                  authz.CapTenantsWrite,
	"acceptFreelanceDelivery":             authz.CapTenantsWrite,
}

func systemGraphQLCapabilityForField(fieldName string) (string, bool) {
	fieldName = strings.TrimSpace(fieldName)
	if fieldName == "" {
		return "", false
	}
	for _, b := range authz.DefaultOperationBindings() {
		if b.Surface != "system_graphql" {
			continue
		}
		if b.Operation == fieldName {
			return b.Capability, true
		}
	}
	if cap, ok := systemGraphQLFieldAliases[fieldName]; ok {
		return cap, true
	}
	return "", false
}

// RequireSystemGraphQLCapabilities enforces apt_ capabilities for system GraphQL
// root fields. When operationName is supplied, only that selected operation is
// checked; this matches GraphQL execution for documents containing multiple
// named operations.
func RequireSystemGraphQLCapabilities(ctx echo.Context, query string, operationName ...string) error {
	if PrincipalFromEcho(ctx) == nil {
		return nil
	}
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("GraphQL query is required")
	}
	doc, err := parser.ParseQuery(&ast.Source{Input: query})
	if err != nil {
		return err
	}
	operations := doc.Operations
	if len(operationName) > 0 && strings.TrimSpace(operationName[0]) != "" {
		op := doc.Operations.ForName(operationName[0])
		if op == nil {
			return fmt.Errorf("unknown GraphQL operation %q", operationName[0])
		}
		operations = ast.OperationList{op}
	}
	if len(operations) == 0 {
		return fmt.Errorf("at least one GraphQL operation is required")
	}
	visiting := make(map[string]bool)
	var walk func(ast.SelectionSet) error
	walk = func(selectionSet ast.SelectionSet) error {
		for _, selection := range selectionSet {
			switch node := selection.(type) {
			case *ast.Field:
				cap, known := systemGraphQLCapabilityForField(node.Name)
				if !known {
					return fmt.Errorf("unknown system GraphQL operation %q", node.Name)
				}
				if err := RequireCapability(ctx, cap); err != nil {
					return err
				}
			case *ast.FragmentSpread:
				fragment := doc.Fragments.ForName(node.Name)
				if fragment == nil {
					return fmt.Errorf("unknown fragment %s", node.Name)
				}
				if visiting[node.Name] {
					return fmt.Errorf("cyclic fragment %s", node.Name)
				}
				visiting[node.Name] = true
				if err := walk(fragment.SelectionSet); err != nil {
					return err
				}
				delete(visiting, node.Name)
			case *ast.InlineFragment:
				if err := walk(node.SelectionSet); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported system GraphQL selection")
			}
		}
		return nil
	}
	for _, op := range operations {
		if err := walk(op.SelectionSet); err != nil {
			return err
		}
	}
	return nil
}

// bootstrapSystemOperations are the only system GraphQL root fields a non-admin
// project API key may call. Everything else fails closed.
var bootstrapSystemOperations = map[string]struct{}{
	"loginUser":             {},
	"registerUser":          {},
	"googleOAuthState":      {},
	"oauthState":            {},
	"searchTenantsByDomain": {},
	"clientRuntime":         {},
}

var bootstrapSystemOperationKinds = map[string]ast.Operation{
	"loginUser":             ast.Query,
	"registerUser":          ast.Mutation,
	"googleOAuthState":      ast.Query,
	"oauthState":            ast.Query,
	"searchTenantsByDomain": ast.Query,
	"clientRuntime":         ast.Query,
}

// ErrSystemOperationForbiddenForClientKey is returned when a non-admin ak_ key
// calls a system GraphQL field outside the authentication allowlist.
var ErrSystemOperationForbiddenForClientKey = fmt.Errorf("SYSTEM_OPERATION_FORBIDDEN_FOR_CLIENT_KEY")

// EnforceBootstrapSystemAllowlist default-denies system GraphQL for non-admin
// project API keys. apt_ capabilities and console admin sessions are unchanged.
func EnforceBootstrapSystemAllowlist(ctx echo.Context, query string) error {
	if ctx == nil {
		return nil
	}
	plane, _ := ctx.Get("auth_plane").(string)
	if plane != "project_api_key" {
		return nil
	}
	role, _ := ctx.Get("role").(string)
	isProjectUser, _ := ctx.Get("is_project_user").(bool)
	if !isProjectUser {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "admin", "owner":
			return nil
		}
	}
	if strings.TrimSpace(query) == "" {
		return ErrSystemOperationForbiddenForClientKey
	}
	doc, err := parser.ParseQuery(&ast.Source{Input: query})
	if err != nil {
		return err
	}
	// Bootstrap project keys use a deliberately small, auditable grammar.
	// Reject fragment definitions and spreads instead of expanding them: this
	// keeps aliases, nesting, and fragment cycles out of the allowlist parser.
	if len(doc.Fragments) > 0 {
		return ErrSystemOperationForbiddenForClientKey
	}
	if len(doc.Operations) != 1 {
		return fmt.Errorf("exactly one system GraphQL operation is required")
	}
	names, err := systemRootFieldNames(doc)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return ErrSystemOperationForbiddenForClientKey
	}
	for _, name := range names {
		if _, ok := bootstrapSystemOperations[name]; !ok {
			return ErrSystemOperationForbiddenForClientKey
		}
		if kind := bootstrapSystemOperationKinds[name]; kind != doc.Operations[0].Operation {
			return ErrSystemOperationForbiddenForClientKey
		}
	}
	return nil
}

func systemRootFieldNames(doc *ast.QueryDocument) ([]string, error) {
	var names []string
	var walk func(ast.SelectionSet, bool) error
	walk = func(set ast.SelectionSet, root bool) error {
		for _, selection := range set {
			switch node := selection.(type) {
			case *ast.Field:
				if len(node.Directives) > 0 {
					return ErrSystemOperationForbiddenForClientKey
				}
				// gqlparser populates Alias with the field name even when no
				// alias was written. Only a differing Alias is user syntax.
				if node.Alias != "" && node.Alias != node.Name {
					return ErrSystemOperationForbiddenForClientKey
				}
				if strings.HasPrefix(node.Name, "__") {
					return ErrSystemOperationForbiddenForClientKey
				}
				if root {
					names = append(names, node.Name)
				}
				if err := walk(node.SelectionSet, false); err != nil {
					return err
				}
			case *ast.FragmentSpread:
				_ = node
				return ErrSystemOperationForbiddenForClientKey
			case *ast.InlineFragment:
				_ = node
				return ErrSystemOperationForbiddenForClientKey
			default:
				return ErrSystemOperationForbiddenForClientKey
			}
		}
		return nil
	}
	if len(doc.Operations) == 0 {
		return nil, fmt.Errorf("at least one GraphQL operation is required")
	}
	for _, operation := range doc.Operations {
		if len(operation.Directives) > 0 {
			return nil, ErrSystemOperationForbiddenForClientKey
		}
		if err := walk(operation.SelectionSet, true); err != nil {
			return nil, err
		}
	}
	return names, nil
}

// ValidateSystemGraphQLCapabilityBindings verifies that every registered
// system root has an explicit capability binding or alias. It is intentionally
// called against the assembled schema maps, rather than a copied list, so a
// newly added resolver cannot silently become an unclassified apt_ operation.
func ValidateSystemGraphQLCapabilityBindings(fieldNames []string) error {
	for _, fieldName := range fieldNames {
		if _, ok := systemGraphQLCapabilityForField(fieldName); !ok {
			return fmt.Errorf("system GraphQL root %q has no capability binding", fieldName)
		}
	}
	return nil
}

// RequireSecuredRESTCapability applies coarse method/path capability gates to
// apt_ secured REST and file routes.
func RequireSecuredRESTCapability(ctx echo.Context, requestPath, method string) error {
	if PrincipalFromEcho(ctx) == nil {
		return nil
	}
	path := strings.ToLower(strings.TrimSpace(requestPath))
	method = strings.ToUpper(strings.TrimSpace(method))
	switch {
	case strings.HasPrefix(path, "/secured/files/"), strings.HasPrefix(path, "/secured/upload/"):
		switch method {
		case "GET", "HEAD":
			return RequireCapability(ctx, authz.CapFilesRead)
		case "DELETE":
			return RequireCapability(ctx, authz.CapFilesDelete)
		default:
			return RequireCapability(ctx, authz.CapFilesWrite)
		}
	case strings.HasPrefix(path, "/secured/rest/"):
		switch method {
		case "GET", "HEAD":
			return RequireCapability(ctx, authz.CapDataRead)
		case "DELETE":
			return RequireCapability(ctx, authz.CapDataDelete)
		default:
			return RequireCapability(ctx, authz.CapDataWrite)
		}
	default:
		return nil
	}
}

// CapabilityDeniedError is returned to MCP/API clients.
type CapabilityDeniedError struct {
	Capability string
}

func (e *CapabilityDeniedError) Error() string {
	return fmt.Sprintf("CAPABILITY_DENIED: missing capability %s", e.Capability)
}

func auditAccessDecision(ctx echo.Context, p *models.AccessPrincipal, capability, decision string) {
	if p == nil {
		return
	}
	// High-signal ops only in structured log for now.
	if !isAuditedCapability(capability) && decision == "allow" {
		return
	}
	projectID := ""
	tenantID := ""
	ip := ""
	ua := ""
	if ctx != nil {
		if v := ctx.Get("project"); v != nil {
			if s, ok := v.(string); ok {
				projectID = s
			}
		}
		if v := ctx.Get("project_id"); v != nil {
			if s, ok := v.(string); ok && projectID == "" {
				projectID = s
			}
		}
		if v := ctx.Get("tenant_id"); v != nil {
			if s, ok := v.(string); ok {
				tenantID = s
			}
		}
		ip = ctx.RealIP()
		ua = ctx.Request().UserAgent()
	}
	log.Printf("[access-audit] decision=%s token=%s issuer=%s project=%s tenant=%s capability=%s ip=%s ua=%q",
		decision, p.TokenID, p.IssuerUserID, projectID, tenantID, capability, ip, ua)
}

func isAuditedCapability(cap string) bool {
	switch cap {
	case authz.CapSchemaPublish, authz.CapFunctionsDeploy, authz.CapPluginsDeploy,
		authz.CapDataDelete, authz.CapTenantsDelete, authz.CapFunctionsDelete,
		authz.CapFilesDelete, authz.CapDatabaseWrite, authz.CapDatabaseRead,
		authz.CapSyncWrite, authz.CapSettingsWrite:
		return true
	default:
		return false
	}
}

func safePath(ctx echo.Context) string {
	if ctx == nil || ctx.Request() == nil || ctx.Request().URL == nil {
		return ""
	}
	return ctx.Request().URL.Path
}

// EnforceProjectForPrincipal runs project grant check when apt_ principal is present.
func EnforceProjectForPrincipal(ctx echo.Context, svc *AccessTokenService, projectID string) error {
	p := PrincipalFromEcho(ctx)
	if p == nil || svc == nil {
		return nil
	}
	if err := svc.AuthorizeProject(ctx.Request().Context(), p, projectID); err != nil {
		auditAccessDecision(ctx, p, "projects.access", "deny")
		if AccessPolicyModeFromEnv() == AccessPolicyReportOnly {
			log.Printf("[access-policy:report-only] project deny token=%s project=%s err=%v", p.TokenID, projectID, err)
			return nil
		}
		return err
	}
	return nil
}

// EnforceTenantForPrincipal runs tenant grant check when apt_ principal is present and tenant is set.
func EnforceTenantForPrincipal(ctx echo.Context, svc *AccessTokenService, projectID, tenantID string) error {
	p := PrincipalFromEcho(ctx)
	if p == nil || svc == nil {
		return nil
	}
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return nil
	}
	if err := svc.AuthorizeTenant(p, projectID, tenantID); err != nil {
		auditAccessDecision(ctx, p, "tenants.access", "deny")
		if AccessPolicyModeFromEnv() == AccessPolicyReportOnly {
			log.Printf("[access-policy:report-only] tenant deny token=%s project=%s tenant=%s err=%v", p.TokenID, projectID, tenantID, err)
			return nil
		}
		return err
	}
	return nil
}

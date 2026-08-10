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

// systemGraphQLFieldAliases maps GraphQL root field names that differ from
// DefaultOperationBindings Operation values (or need CapRolesRead fallbacks).
var systemGraphQLFieldAliases = map[string]string{
	"upsertRoleToProject":      authz.CapRolesWrite,
	"deleteRoleFromProject":    authz.CapRolesWrite,
	"duplicateRoleInProject":   authz.CapRolesWrite,
	"deleteRole":               authz.CapRolesWrite,
	"getProjectPlans":          authz.CapPlansRead,
	"upsertPlanToProject":      authz.CapPlansWrite,
	"duplicatePlanInProject":   authz.CapPlansWrite,
	"deletePlanFromProject":    authz.CapPlansWrite,
	"generateProjectToken":     authz.CapProjectsWrite,
	"deleteProjectToken":       authz.CapProjectsWrite,
	"currentProject":           authz.CapProjectsRead,
	"getProjectRoles":          authz.CapRolesRead,
	"listPermissionsAndScopes": authz.CapRolesRead,
}

func systemGraphQLCapabilityForField(fieldName string) string {
	fieldName = strings.TrimSpace(fieldName)
	if fieldName == "" {
		return ""
	}
	for _, b := range authz.DefaultOperationBindings() {
		if b.Surface != "system_graphql" {
			continue
		}
		if b.Operation == fieldName {
			return b.Capability
		}
	}
	if cap, ok := systemGraphQLFieldAliases[fieldName]; ok {
		// Prefer CapRolesRead when bound for role-list fields; alias already uses CapRolesRead.
		// If CapRolesRead were unavailable as a concept, fall back to CapProjectsRead.
		if fieldName == "getProjectRoles" || fieldName == "listPermissionsAndScopes" {
			if _, ok := authz.Get(authz.CapRolesRead); ok {
				return authz.CapRolesRead
			}
			return authz.CapProjectsRead
		}
		return cap
	}
	return ""
}

// RequireSystemGraphQLCapabilities enforces apt_ capabilities for system GraphQL root fields.
func RequireSystemGraphQLCapabilities(ctx echo.Context, query string) error {
	if PrincipalFromEcho(ctx) == nil || strings.TrimSpace(query) == "" {
		return nil
	}
	doc, err := parser.ParseQuery(&ast.Source{Input: query})
	if err != nil {
		return err
	}
	if len(doc.Operations) == 0 {
		return fmt.Errorf("at least one GraphQL operation is required")
	}
	for _, op := range doc.Operations {
		for _, selection := range op.SelectionSet {
			field, ok := selection.(*ast.Field)
			if !ok {
				continue
			}
			cap := systemGraphQLCapabilityForField(field.Name)
			if cap == "" {
				continue
			}
			if err := RequireCapability(ctx, cap); err != nil {
				return err
			}
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

package resolver

import (
	"errors"
	"strings"
	"sync"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/scaler"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

var (
	effectivePermissionsPayloadOnce sync.Once
	effectivePermissionsPayloadObj  *graphql.Object
)

func effectivePermissionsPayloadType() *graphql.Object {
	effectivePermissionsPayloadOnce.Do(func() {
		effectivePermissionsPayloadObj = graphql.NewObject(graphql.ObjectConfig{
			Name: "MyEffectivePermissionsPayload",
			Fields: graphql.Fields{
				"plan_slug":        &graphql.Field{Type: graphql.String},
				"role_id":          &graphql.Field{Type: graphql.String},
				"plan_clamped":     &graphql.Field{Type: graphql.Boolean},
				"api_permissions":  &graphql.Field{Type: scaler.ScalarJSON},
				"logic_executions": &graphql.Field{Type: graphql.NewList(graphql.String)},
				"quotas":           &graphql.Field{Type: scaler.ScalarJSON},
				"usage":            &graphql.Field{Type: scaler.ScalarJSON},
				"grace_models":     &graphql.Field{Type: graphql.NewList(graphql.String)},
				"is_admin":         &graphql.Field{Type: graphql.Boolean},
			},
		})
	})
	return effectivePermissionsPayloadObj
}

// MyEffectivePermissionsResolverFn returns the request-scoped role after plan clamp,
// plus plan slug, quotas/usage, and per-model read-grace flags.
//
// Public GraphQL injects ApplicationCache on the resolve context (same as model
// resolvers). Prefer that over GetApplicationCache(router) so auth-only queries
// stay aligned with the secured/public execution path.
func (s *GraphQLServer) MyEffectivePermissionsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	cache, ok := utility.LegacyApplicationCache(p.Context)
	if !ok || cache == nil {
		// Fallback for callers that only set echo router (e.g. tests / system wraps).
		if router, rok := p.Context.Value("router").(echo.Context); rok && router != nil {
			var err error
			cache, err = s.GetApplicationCache(router)
			if err != nil {
				return nil, err
			}
		}
	}
	if cache == nil || cache.Param == nil || cache.Param.Role == nil {
		return nil, errors.New("authenticated app-user context required")
	}

	isProjectUser := false
	if router, rok := p.Context.Value("router").(echo.Context); rok && router != nil {
		isProjectUser, _ = router.Get("is_project_user").(bool)
	}
	if !isProjectUser {
		isProjectUser = cache.Param.Role.IsProjectUser || cache.Param.Role.PlanClamped
	}
	if !isProjectUser {
		return nil, errors.New("myEffectivePermissions is for app-user tokens")
	}

	role := cache.Param.Role
	plan := cache.Param.ActivePlan
	planSlug := strings.TrimSpace(cache.Param.Plan)
	if planSlug == "" && cache.Param.Ext != nil {
		if s, ok := cache.Param.Ext["plan_slug"].(string); ok {
			planSlug = s
		}
	}

	// Prefer pre-clamp role from project for grace computation when plan present.
	var baseRole *models.Role
	if cache.Project != nil && cache.Project.Roles != nil && role.ID != "" {
		if r, ok := cache.Project.Roles[role.ID]; ok && r != nil {
			baseRole = r
		}
	}
	if baseRole == nil {
		baseRole = role
	}
	grace := utility.ComputeReadGraceByModel(baseRole, plan)

	perms := make(map[string]interface{})
	if role.APIPermissions != nil {
		for k, v := range role.APIPermissions {
			if v == nil {
				continue
			}
			entry := map[string]interface{}{
				"read": v.Read, "create": v.Create, "update": v.Update, "delete": v.Delete,
			}
			if grace[k] {
				entry["grace"] = true
			}
			perms[k] = entry
		}
	}

	graceModels := make([]string, 0, len(grace))
	for k, v := range grace {
		if v {
			graceModels = append(graceModels, k)
		}
	}

	quotas := map[string]interface{}{}
	usage := map[string]interface{}{}
	if plan != nil && plan.Quotas != nil && cache.Param != nil {
		for k, limit := range plan.Quotas {
			quotas[k] = limit
			if !strings.HasPrefix(k, utility.QuotaKeyRecordsPrefix) {
				continue
			}
			modelName := strings.TrimPrefix(k, utility.QuotaKeyRecordsPrefix)
			if modelName == "" {
				continue
			}
			// Best-effort usage; never fail the query on count errors / missing driver.
			func() {
				defer func() { _ = recover() }()
				count, err := s.cachedModelDocCount(p.Context, cache, cache.Param, modelName)
				if err == nil {
					usage[k] = count
				}
			}()
		}
	}

	logic := []string{}
	if role.LogicExecutions != nil {
		logic = append(logic, role.LogicExecutions...)
	}

	return map[string]interface{}{
		"plan_slug":        planSlug,
		"role_id":          role.ID,
		"plan_clamped":     role.PlanClamped,
		"api_permissions":  perms,
		"logic_executions": logic,
		"quotas":           quotas,
		"usage":            usage,
		"grace_models":     graceModels,
		"is_admin":         role.IsAdmin,
	}, nil
}

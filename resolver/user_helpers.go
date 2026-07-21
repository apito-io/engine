package resolver

import (
	"context"
	"errors"
	"strings"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/services"
	"github.com/labstack/echo/v4"
)

func (s *GraphQLServer) systemDB() interfaces.ApitoSystemDB {
	if s == nil || s.SystemDriver == nil {
		return nil
	}
	return s.SystemDriver
}

// RequireProjectAdmin ensures the caller has project admin role.
func RequireProjectAdmin(cache *models.ApplicationCache) error {
	return requireProjectAdmin(cache)
}

func requireProjectAdmin(cache *models.ApplicationCache) error {
	if cache == nil || cache.Param == nil || cache.Param.Role == nil {
		return errors.New("admin role required")
	}
	if cache.Param.Role.IsAdmin || strings.EqualFold(strings.TrimSpace(cache.Param.Role.ID), "admin") {
		return nil
	}
	return errors.New("admin role required")
}

// requireFunctionManage gates function list/upsert/delete/test/deploy/history/rollback.
// Today: owner/admin or project_admin. Centralized so a future logic.manage team
// permission can be added without touching each resolver.
// When an apt_ AccessPrincipal is on the echo context, also require functions.write
// (callers that need deploy/test/delete should call requireAccessCapability separately).
func requireFunctionManage(cache *models.ApplicationCache) error {
	if cache == nil || cache.Param == nil || cache.Param.Role == nil {
		return errors.New("function management requires project admin")
	}
	role := cache.Param.Role
	id := strings.ToLower(strings.TrimSpace(role.ID))
	if role.IsAdmin || id == "admin" || id == "owner" || id == "project_admin" {
		return nil
	}
	// Future seam: if role.Permissions contains "logic.manage", allow.
	return errors.New("function management requires project admin")
}

// RequireFunctionManage is the exported form of requireFunctionManage.
func RequireFunctionManage(cache *models.ApplicationCache) error {
	return requireFunctionManage(cache)
}

// requireAccessCapability enforces apt_ capability when principal present (no-op for cookie sessions).
func requireAccessCapability(router echo.Context, capability string) error {
	return services.RequireCapability(router, capability)
}

// RequireAccessCapability is the exported form.
func RequireAccessCapability(router echo.Context, capability string) error {
	return requireAccessCapability(router, capability)
}

// routerFromResolveParams extracts echo.Context from GraphQL resolve context.
func routerFromResolveParams(ctx context.Context) echo.Context {
	if ctx == nil {
		return nil
	}
	if v := ctx.Value("router"); v != nil {
		if r, ok := v.(echo.Context); ok {
			return r
		}
	}
	return nil
}

// Cap is a convenience re-export for resolvers.
const (
	CapFunctionsRead   = authz.CapFunctionsRead
	CapFunctionsWrite  = authz.CapFunctionsWrite
	CapFunctionsTest   = authz.CapFunctionsTest
	CapFunctionsDeploy = authz.CapFunctionsDeploy
	CapFunctionsDelete = authz.CapFunctionsDelete
	CapSchemaRead      = authz.CapSchemaRead
	CapSchemaWrite     = authz.CapSchemaWrite
	CapSchemaPublish   = authz.CapSchemaPublish
	CapSyncRead        = authz.CapSyncRead
	CapSyncWrite       = authz.CapSyncWrite
	CapDataRead        = authz.CapDataRead
	CapDataWrite       = authz.CapDataWrite
	CapDataDelete      = authz.CapDataDelete
	CapProjectsRead    = authz.CapProjectsRead
	CapProjectsWrite   = authz.CapProjectsWrite
	CapPluginsWrite    = authz.CapPluginsWrite
	CapRolesWrite      = authz.CapRolesWrite
)

// GetArgString reads a string GraphQL argument.
func GetArgString(args map[string]interface{}, key string) string {
	return getArgString(args, key)
}

func getArgString(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}

// GetArgInt reads an int GraphQL argument with a default.
func GetArgInt(args map[string]interface{}, key string, def int) int {
	return getArgInt(args, key, def)
}

func getArgInt(args map[string]interface{}, key string, def int) int {
	if args == nil {
		return def
	}
	v, ok := args[key]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case int:
		return t
	case int32:
		return int(t)
	case int64:
		return int(t)
	case float64:
		return int(t)
	default:
		return def
	}
}

func appUserToMap(u *models.User) map[string]interface{} {
	return models.UserToPublicMap(u)
}

// ResolveNewUserRole returns an explicit role arg when set, otherwise the project's registration default.
func ResolveNewUserRole(project *models.Project, roleArg string) string {
	if r := strings.TrimSpace(roleArg); r != "" {
		return r
	}
	return models.RegistrationDefaultRole(project)
}

// NormalizeUserUsernameArg normalizes an optional create/update username argument.
func NormalizeUserUsernameArg(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// resolveCreateUserUsername picks the stored username: caller value when set, else internal key.
func resolveCreateUserUsername(ctx context.Context, sys interfaces.ApitoSystemDB, projectID, uid, usernameArg string) (string, error) {
	uname := NormalizeUserUsernameArg(usernameArg)
	if uname == "" {
		return internalUserUsername(uid), nil
	}
	ex, err := sys.GetUserByUsername(ctx, projectID, uname)
	if err != nil {
		return "", err
	}
	if ex != nil {
		return "", errors.New("username already exists")
	}
	return uname, nil
}

// internalUserUsername is a non-user-facing unique key per row (SQL uniqueness on project_id, username).
func internalUserUsername(id string) string {
	s := strings.ReplaceAll(strings.TrimSpace(id), "-", "")
	if len(s) > 24 {
		s = s[:24]
	}
	if s == "" {
		s = "x"
	}
	return "u_" + s
}

package resolver

import (
	"context"
	"strings"

	apifn "github.com/apito-io/engine/functions"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// selectDenoProvider always returns the real Deno provider. Missing Deno fails at Invoke.
func selectDenoProvider(gateway apifn.FunctionDataGateway) apifn.RuntimeProvider {
	return &apifn.DenoRuntimeProvider{
		Gateway:  gateway,
		Registry: apifn.GlobalInvocationRegistry,
	}
}

// functionGatewayDeps adapts GraphQLServer to apifn.GatewayDeps.
type functionGatewayDeps struct {
	server *GraphQLServer
}

func (d functionGatewayDeps) GetProjectDriver(ctx context.Context, inv *apifn.InvocationContext) (interfaces.ProjectDBInterface, error) {
	_ = inv
	return d.server.GraphQLExecutor.GetProjectDriver(ctx)
}

func (d functionGatewayDeps) ResolveModel(inv *apifn.InvocationContext, modelArg string) (*models.ModelType, error) {
	if inv == nil || inv.Project == nil || inv.Project.Schema == nil {
		return nil, nil
	}
	return resolveProjectModelFromSchema(inv.Project.Schema.Models, modelArg), nil
}

func (d functionGatewayDeps) NewParam(inv *apifn.InvocationContext) *models.CommonSystemParams {
	if inv == nil || inv.Param == nil {
		return &models.CommonSystemParams{}
	}
	return d.server.NewParam(inv.Param)
}

// WireFunctionDataGateway registers driver-backed read ops on the runtime gateway.
func (s *GraphQLServer) WireFunctionDataGateway() {
	if s == nil || s.FunctionRuntime == nil {
		return
	}
	gw := s.FunctionRuntime.DataGateway()
	eng, ok := gw.(*apifn.EngineDataGateway)
	if !ok || eng == nil {
		return
	}
	apifn.RegisterReadDataOps(eng, functionGatewayDeps{server: s}, apifn.GlobalInvocationRegistry)
}

// registerFunctionInvocation stores trusted host context for Deno bridge / gateway.
// Tenant routing must already be on cache.Ctx via FunctionTenantScopeHook (typed Pro
// keys). Do not inject a plain-string "tenant_id" context value — that key never
// matches proconst.TenantIDContextKey and silently falls back to the base project DB.
func (s *GraphQLServer) registerFunctionInvocation(ctx context.Context, cache *models.ApplicationCache, env *apifn.InvocationEnvelope) error {
	if env == nil || cache == nil {
		return nil
	}
	param := s.NewParam(cache.Param)
	if param.Ext == nil {
		param.Ext = map[string]interface{}{}
	}
	if env.TenantID != "" {
		param.Ext["tenant_id"] = env.TenantID
	}
	dbCtx := publicProjectDBContext(cache, ctx)
	if dbCtx.Value("project_id") == nil && env.ProjectID != "" {
		dbCtx = context.WithValue(dbCtx, "project_id", env.ProjectID)
	}
	inv := &apifn.InvocationContext{
		Envelope:     env,
		DBCtx:        dbCtx,
		Param:        param,
		Project:      cache.Project,
		Capabilities: append([]string{}, env.Capabilities...),
		Gateway:      s.FunctionRuntime.DataGateway(),
	}
	return apifn.GlobalInvocationRegistry.Register(inv)
}

// ApplyFunctionTenantScope runs the host FunctionTenantScopeHook (when set) and
// returns the resolved tenant id. Mutates cache.Ctx / cache.Param in place.
func (s *GraphQLServer) ApplyFunctionTenantScope(
	ctx context.Context,
	cache *models.ApplicationCache,
	mode models.FunctionTenantScopeMode,
	explicitTenantID string,
) (string, error) {
	return s.applyFunctionTenantScope(ctx, cache, mode, explicitTenantID)
}

// applyFunctionTenantScope runs the host FunctionTenantScopeHook (when set) and
// returns the resolved tenant id. Mutates cache.Ctx / cache.Param in place.
func (s *GraphQLServer) applyFunctionTenantScope(
	ctx context.Context,
	cache *models.ApplicationCache,
	mode models.FunctionTenantScopeMode,
	explicitTenantID string,
) (string, error) {
	if s == nil || s.Cfg == nil || s.Cfg.FunctionTenantScopeHook == nil || cache == nil {
		tid := strings.TrimSpace(explicitTenantID)
		if tid == "" && cache != nil && cache.Param != nil && cache.Param.Ext != nil {
			if v, ok := cache.Param.Ext["tenant_id"].(string); ok {
				tid = strings.TrimSpace(v)
			}
		}
		return tid, nil
	}
	return s.Cfg.FunctionTenantScopeHook(ctx, cache, mode, explicitTenantID)
}

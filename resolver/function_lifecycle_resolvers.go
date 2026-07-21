package resolver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	apifn "github.com/apito-io/engine/functions"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

func (s *GraphQLServer) lifecycleStore() interfaces.FunctionLifecycleStore {
	if s == nil || s.SystemDriver == nil {
		return nil
	}
	if p, ok := s.SystemDriver.(interfaces.FunctionLifecycleProvider); ok {
		return p.FunctionLifecycleStore()
	}
	return nil
}

func (s *GraphQLServer) findProjectFunction(project *models.Project, name string) *models.ApitoFunction {
	if project == nil || project.Schema == nil {
		return nil
	}
	for _, f := range project.Schema.Functions {
		if f != nil && f.Name == name {
			return f
		}
	}
	return nil
}

// DeployFunctionToProjectResolverFn snapshots draft Source into an immutable revision and activates it.
func (s *GraphQLServer) DeployFunctionToProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireAccessCapability(router, CapFunctionsDeploy); err != nil {
		return nil, err
	}
	if err := requireFunctionManage(cache); err != nil {
		return nil, err
	}
	name, _ := p.Args["name"].(string)
	name = utility.SingularResourceName(strings.TrimSpace(name))
	if name == "" {
		return nil, errors.New("function name required")
	}
	fn := s.findProjectFunction(cache.Project, name)
	if fn == nil {
		return nil, fmt.Errorf("function %q not found", name)
	}
	fn.ProjectID = cache.Project.ID
	source := fn.Source
	if override, ok := p.Args["source"].(string); ok && strings.TrimSpace(override) != "" {
		source = override
		fn.Source = override
	}
	if strings.TrimSpace(source) == "" {
		return nil, errors.New("empty draft source")
	}
	actor := ""
	if cache.Param != nil {
		actor = cache.Param.UserID
	}
	var artifacts apifn.ArtifactStore
	if s.FunctionRuntime != nil {
		artifacts = s.FunctionRuntime.Artifacts()
	}
	rev, build, dep, err := apifn.DeployDraft(p.Context, artifacts, fn, []byte(source), actor)
	if err != nil {
		return nil, err
	}
	store := s.lifecycleStore()
	if store != nil {
		_ = store.MarkDeploymentsSuperseded(p.Context, fn.ProjectID, fn.Name, "")
		if err := store.SaveRevision(p.Context, rev); err != nil {
			return nil, err
		}
		if build != nil {
			_ = store.SaveBuild(p.Context, build)
		}
		if err := store.SaveDeployment(p.Context, dep); err != nil {
			return nil, err
		}
	}
	if err := s.SystemDriver.UpdateProject(p.Context, cache.Project, false); err != nil {
		return nil, err
	}
	_ = s.ExpireGraphQLProjectCache(p.Context, cache.Project.ID)
	return map[string]interface{}{
		"function":   fn,
		"revision":   rev,
		"deployment": dep,
	}, nil
}

// RollbackFunctionDeploymentResolverFn activates a previous revision for live execution.
func (s *GraphQLServer) RollbackFunctionDeploymentResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireAccessCapability(router, CapFunctionsDeploy); err != nil {
		return nil, err
	}
	if err := requireFunctionManage(cache); err != nil {
		return nil, err
	}
	name, _ := p.Args["name"].(string)
	name = utility.SingularResourceName(strings.TrimSpace(name))
	revisionID, _ := p.Args["revision_id"].(string)
	revisionID = strings.TrimSpace(revisionID)
	if name == "" || revisionID == "" {
		return nil, errors.New("name and revision_id required")
	}
	fn := s.findProjectFunction(cache.Project, name)
	if fn == nil {
		return nil, fmt.Errorf("function %q not found", name)
	}
	fn.ProjectID = cache.Project.ID

	var target *models.FunctionRevision
	store := s.lifecycleStore()
	if store != nil {
		target, err = store.GetRevision(p.Context, cache.Project.ID, revisionID)
		if err != nil {
			return nil, fmt.Errorf("revision not found: %w", err)
		}
	}
	if target == nil {
		// Artifact-only fallback when lifecycle SQL is unavailable.
		key := fmt.Sprintf("%s/%s/%s", cache.Project.ID, name, revisionID)
		var artifacts apifn.ArtifactStore
		if s.FunctionRuntime != nil {
			artifacts = s.FunctionRuntime.Artifacts()
		}
		if artifacts == nil {
			return nil, errors.New("revision store unavailable")
		}
		data, hash, err := artifacts.Get(p.Context, key)
		if err != nil {
			return nil, fmt.Errorf("revision artifact not found: %w", err)
		}
		target = &models.FunctionRevision{
			ID:           revisionID,
			ProjectID:    cache.Project.ID,
			Name:         name,
			Source:       string(data),
			ArtifactKey:  key,
			ArtifactHash: hash,
		}
	}
	actor := ""
	if cache.Param != nil {
		actor = cache.Param.UserID
	}
	dep := apifn.RollbackDeployment(fn, target, actor)
	if store != nil && dep != nil {
		_ = store.MarkDeploymentsSuperseded(p.Context, fn.ProjectID, fn.Name, "")
		_ = store.SaveDeployment(p.Context, dep)
	}
	if err := s.SystemDriver.UpdateProject(p.Context, cache.Project, false); err != nil {
		return nil, err
	}
	_ = s.ExpireGraphQLProjectCache(p.Context, cache.Project.ID)
	return map[string]interface{}{
		"function":   fn,
		"revision":   target,
		"deployment": dep,
	}, nil
}

// TestFunctionDraftResolverFn runs the editable draft (or override source) synchronously for admins.
// Does not require or reveal X-Fn-Hash.
func (s *GraphQLServer) TestFunctionDraftResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireAccessCapability(router, CapFunctionsTest); err != nil {
		return nil, err
	}
	if err := requireFunctionManage(cache); err != nil {
		return nil, err
	}
	name, _ := p.Args["name"].(string)
	name = utility.SingularResourceName(strings.TrimSpace(name))
	if name == "" {
		return nil, errors.New("function name required")
	}
	fn := s.findProjectFunction(cache.Project, name)
	if fn == nil {
		return nil, fmt.Errorf("function %q not found", name)
	}
	if !fn.IsApitoFunctionsRuntime() {
		return nil, errors.New("draft test only supported for apito functions runtimes")
	}
	if s.FunctionRuntime == nil {
		return nil, errors.New("function runtime not configured")
	}

	source := fn.Source
	if override, ok := p.Args["source"].(string); ok && strings.TrimSpace(override) != "" {
		source = override
	}
	if strings.TrimSpace(source) == "" {
		return nil, errors.New("empty draft source")
	}

	payload := map[string]interface{}{}
	if raw, ok := p.Args["payload"]; ok && raw != nil {
		switch t := raw.(type) {
		case map[string]interface{}:
			payload = t
		case string:
			_ = json.Unmarshal([]byte(t), &payload)
		}
	}
	explicitTenant := ""
	if tid, ok := p.Args["tenant_id"].(string); ok {
		explicitTenant = strings.TrimSpace(tid)
	}

	testFn := *fn
	testFn.Source = source
	testFn.ActiveRevisionID = "" // force draft path
	testFn.BinaryURL = ""
	testFn.ProjectID = cache.Project.ID

	role := ""
	userID := ""
	if cache.Param != nil {
		userID = cache.Param.UserID
		if cache.Param.Role != nil {
			role = cache.Param.Role.ID
		}
	}

	testCache := &models.ApplicationCache{
		Project: cache.Project,
		Param:   s.NewParam(cache.Param),
		Ctx:     cache.Ctx,
	}
	tenantID, err := s.applyFunctionTenantScope(p.Context, testCache, models.FunctionTenantScopeDraftTest, explicitTenant)
	if err != nil {
		out := map[string]interface{}{
			"ok":            false,
			"duration_ms":   int64(0),
			"invocation_id": "",
			"logs":          []interface{}{},
			"error":         err.Error(),
		}
		if te, ok := apifn.AsTenantScopeError(err); ok {
			out["error_class"] = te.Code
			out["error"] = te.Message
			if te.Message == "" {
				out["error"] = te.Code
			}
		}
		return out, nil
	}

	invocationID := utility.NewID()
	env := apifn.BuildEnvelope(&testFn, cache.Project.ID, tenantID, userID, role, payload, invocationID)
	env.Source = source

	if err := s.registerFunctionInvocation(p.Context, testCache, env); err != nil {
		return nil, err
	}
	defer apifn.GlobalInvocationRegistry.Unregister(env.InvocationID)

	start := time.Now()
	result, err := s.FunctionRuntime.Invoke(p.Context, env)
	dur := time.Since(start).Milliseconds()
	s.persistInvocationSummary(p.Context, &testFn, tenantID, userID, result, dur, err)

	out := map[string]interface{}{
		"ok":            false,
		"duration_ms":   dur,
		"invocation_id": invocationID,
		"logs":          []interface{}{},
	}
	if err != nil {
		out["error"] = err.Error()
		return out, nil
	}
	if result == nil {
		out["error"] = "empty result"
		return out, nil
	}
	out["ok"] = result.OK
	out["response"] = result.Response
	out["error"] = result.Error
	out["error_class"] = result.ErrorClass
	if result.DurationMs > 0 {
		out["duration_ms"] = result.DurationMs
	}
	logs := make([]map[string]interface{}, 0, len(result.Logs))
	for _, l := range result.Logs {
		logs = append(logs, map[string]interface{}{
			"level":   l.Level,
			"message": l.Message,
		})
	}
	out["logs"] = logs
	return out, nil
}

func (s *GraphQLServer) persistInvocationSummary(ctx context.Context, fn *models.ApitoFunction, tenantID, principal string, result *apifn.InvocationResult, dur int64, invokeErr error) {
	store := s.lifecycleStore()
	if store == nil || fn == nil {
		return
	}
	now := time.Now().UTC().Format(time.RFC3339)
	inv := &models.FunctionInvocation{
		ID:          utility.NewID(),
		ProjectID:   fn.ProjectID,
		Name:        fn.Name,
		RevisionID:  fn.ActiveRevisionID,
		TenantScope: tenantID,
		Principal:   principal,
		Status:      "committed",
		DurationMs:  dur,
		CreatedAt:   now,
		CompletedAt: now,
	}
	if invokeErr != nil {
		inv.Status = "failed"
		inv.ErrorClass = "abort"
	} else if result != nil {
		inv.DurationMs = result.DurationMs
		inv.ErrorClass = result.ErrorClass
		if !result.OK {
			inv.Status = "failed"
		}
		if len(result.Logs) > 0 {
			b, _ := json.Marshal(result.Logs)
			if len(b) > 8*1024 {
				b = b[:8*1024]
			}
			inv.Logs = string(b)
		}
	}
	_ = store.SaveInvocation(ctx, inv)
}

// ListFunctionRevisionsResolverFn returns immutable revisions for a function.
func (s *GraphQLServer) ListFunctionRevisionsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireFunctionManage(cache); err != nil {
		return nil, err
	}
	name, _ := p.Args["name"].(string)
	name = utility.SingularResourceName(strings.TrimSpace(name))
	store := s.lifecycleStore()
	if store == nil {
		return []*models.FunctionRevision{}, nil
	}
	limit := 50
	if n, ok := p.Args["limit"].(int); ok && n > 0 {
		limit = n
	}
	return store.ListRevisions(p.Context, cache.Project.ID, name, limit)
}

// ListFunctionDeploymentsResolverFn returns deployment history.
func (s *GraphQLServer) ListFunctionDeploymentsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	if err := requireFunctionManage(cache); err != nil {
		return nil, err
	}
	name, _ := p.Args["name"].(string)
	name = utility.SingularResourceName(strings.TrimSpace(name))
	store := s.lifecycleStore()
	if store == nil {
		return []*models.FunctionDeployment{}, nil
	}
	limit := 50
	if n, ok := p.Args["limit"].(int); ok && n > 0 {
		limit = n
	}
	return store.ListDeployments(p.Context, cache.Project.ID, name, limit)
}

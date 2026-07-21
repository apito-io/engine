package resolver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/apito-io/engine/database/helper"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/services"
	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/apito-io/types/protobuff"
	"github.com/elliotchance/pie/pie"
	"github.com/labstack/echo/v4"
	graphqlClient "github.com/machinebox/graphql"
	"github.com/tailor-platform/graphql"
)

/*func (s *GraphQLServer) SwitchProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {
}*/

/* func (s *GraphQLServer) EnterProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	s.Lock()
	defer s.Unlock()

	var projectId string
	if val, ok := p.Args["id"].(string); ok {
		projectId = val
	} else {
		return nil, ae.ProjectIdRequired
	}

	var token string
	if val, ok := p.Args["token"].(string); ok {
		token = val
	} else {
		return nil, ae.TokenIsRequired
	}

	user, err := s.SystemDriver.GetSystemUser(p.Context, param.UserID)
	if err != nil {
		return nil, utility.CaptureInternalServerError(err, map[string]interface{}{
			"req": param,
		})
	}

	user.CurrentProjectID = projectId
	err = s.SystemDriver.UpdateSystemUser(p.Context, user, false)
	if err != nil {
		return nil, err
	}

	// refresh the token
	/* refreshTokens, err := utility.NewRefreshTokenAuthenticator(s.Cfg, token)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		//"token": refreshTokens.IDToken,
		"token": token,
	}, nil
} */

func (s *GraphQLServer) ConnectSupportResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	hybridAuthCallToken := "EhltDOqqL0sKV1z38iDQsfQfqQluFWvTfBeFrdmAleirMG0dTrvfqi3qtgaSNx7Zf1aeP49lDnUr6shDgOca8R7RsK0sNRYetvoKguNzgpx4Er4RxSUl2Y1YF6GcIEPgbDTAfv3ltA7JPyHZDJHlbWZjEjbMV24DOKt3ChFCmjA0yUwXMCf6f6e"

	// create a GraphQL client (safe to share across requests)
	client := graphqlClient.NewClient("https://api.apito.io/secured/graphql")

	// make a request
	req := graphqlClient.NewRequest(fmt.Sprintf(`
		   mutation MyMutation {
			  hybridAuth(payload: {
				user_id : "%s", 
				email: "%s" 
			  }) {
				JSON
			  }
			}`, param.UserID, param.Email))

	// set header fields
	req.Header.Set("Authorization", "Bearer "+hybridAuthCallToken)

	// run it and capture the response
	var respData map[string]interface{}
	if err := client.Run(context.Background(), req, &respData); err != nil {
		return nil, err
	}

	if resp, ok := respData["hybridAuth"].(map[string]interface{}); ok {
		if json, ok := resp["JSON"].(map[string]interface{}); ok {
			if login, ok := json["userLogin"].(map[string]interface{}); ok {
				return login, nil
			}
		}
	}

	return nil, nil
}

func (s *GraphQLServer) ListProjectsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	param.ResolveParams = &p
	param.SystemCollectionName = "projects"

	res, err := s.SystemDriver.SearchProjects(p.Context, param)
	if err != nil {
		return nil, err
	}

	results := res.Results
	if principal := services.PrincipalFromEcho(router); principal != nil {
		if err := requireAccessCapability(router, CapProjectsRead); err != nil {
			return nil, err
		}
		tokenSvc := s.ApitoTokenService.AccessTokens()
		filtered := make([]*models.Project, 0, len(results))
		for _, project := range results {
			if project == nil || strings.TrimSpace(project.ID) == "" {
				continue
			}
			if err := tokenSvc.AuthorizeProject(p.Context, principal, project.ID); err == nil {
				filtered = append(filtered, project)
			}
		}
		return filtered, nil
	}
	if merged, err := s.mergeSyncTokenProjects(p.Context, router, results); err != nil {
		return nil, err
	} else if merged != nil {
		results = merged
	}

	return results, nil
}

func (s *GraphQLServer) mergeSyncTokenProjects(ctx context.Context, router echo.Context, results []*models.Project) ([]*models.Project, error) {
	var tokenProjectIDs []string
	if raw := router.Get("project_ids"); raw != nil {
		if ids, ok := raw.([]string); ok {
			tokenProjectIDs = ids
		}
	}
	if len(tokenProjectIDs) == 0 {
		if raw := router.Get("sync_token_claims"); raw != nil {
			if claims, ok := raw.(*models.TokenClaims); ok && len(claims.ProjectIDs) > 0 {
				tokenProjectIDs = claims.ProjectIDs
			}
		}
	}
	if len(tokenProjectIDs) == 0 {
		return results, nil
	}

	seen := make(map[string]struct{}, len(results)+len(tokenProjectIDs))
	for _, p := range results {
		if p != nil && p.ID != "" {
			seen[p.ID] = struct{}{}
		}
	}

	merged := results
	for _, pid := range tokenProjectIDs {
		if pid == "" {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		proj, err := s.SystemDriver.GetProject(ctx, pid)
		if err != nil || proj == nil {
			continue
		}
		merged = append(merged, proj)
		seen[pid] = struct{}{}
	}
	return merged, nil
}

func (s *GraphQLServer) ListAllProjectsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	param.ResolveParams = &p
	param.SystemCollectionName = "projects"

	return s.SystemDriver.SearchProjects(p.Context, param)
}

/*func (s *GraphQLServer) ListAllUsersResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()
	return s.SystemDriver.ListAllUsers(p.Context)
}*/

func (s *GraphQLServer) ListRoleScopesResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	if cache.Project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	var _models []string
	for _, m := range cache.Project.Schema.Models {
		_models = append(_models, m.Name)
	}

	/*
		if _, ok := s.Extensions["aws"]; ok {
			administrations = append(administrations, "logic")
		}*/

	/*	if s.AddOns.Auth != nil {
		administrations = append(administrations, "user")
	}*/

	var _functions []string
	for _, m := range cache.Project.Schema.Functions {
		_functions = append(_functions, m.Name)
	}

	return map[string]interface{}{
		"permissions": models.GlobalPermissions,
		"models":      _models,
		"functions":   _functions,
	}, nil
}

func (s *GraphQLServer) GetCurrentProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapProjectsRead); err != nil {
		return nil, err
	}

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := new(models.Project)

	*project = *cache.Project
	project.Schema = nil

	return project, nil
}

func parseGraphQLStringListArg(args map[string]interface{}, key string) []string {
	raw, ok := args[key]
	if !ok || raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					out = append(out, s)
				}
			}
		}
		return out
	case []string:
		out := make([]string, 0, len(v))
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func cloneResolveParamsForModelCount(parent graphql.ResolveParams, modelName string) graphql.ResolveParams {
	child := parent
	child.Args = make(map[string]interface{}, len(parent.Args)+2)
	for k, v := range parent.Args {
		child.Args[k] = v
	}
	child.Args["model"] = modelName
	child.Args["status"] = "all"
	return child
}

// isMissingDatastoreTableErr reports driver errors when the physical model table was never created
// (schema published in system DB but project DB not migrated yet). Used by sidebar count badges only.
func isMissingDatastoreTableErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "no such table") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "doesn't exist")
}

func (s *GraphQLServer) ModelDocumentCountsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapDataRead); err != nil {
		return nil, err
	}

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	if cache.Project.Schema == nil {
		return []map[string]interface{}{}, nil
	}

	requested := parseGraphQLStringListArg(p.Args, "models")
	modelNames := requested
	if len(modelNames) == 0 {
		for _, m := range cache.Project.Schema.Models {
			if m == nil || m.SystemGenerated || strings.TrimSpace(m.Name) == "" {
				continue
			}
			modelNames = append(modelNames, m.Name)
		}
	}
	sort.Strings(modelNames)

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	out := make([]map[string]interface{}, 0, len(modelNames))
	for _, modelArg := range modelNames {
		modelType := resolveProjectModelFromSchema(cache.Project.Schema.Models, modelArg)
		if modelType == nil {
			return nil, ae.ModelTypeNotFound
		}

		countParams := cloneResolveParamsForModelCount(p, modelType.Name)
		param := s.NewParam(cache.Param)
		param.Model = modelType
		param.ResolveParams = &countParams
		param.ProjectSchemaModels = cache.Project.Schema.Models
		param.IsSystemRequest = true

		count, err := driver.CountMultiDocumentOfProject(cache.Ctx, param, false)
		if err != nil {
			if isMissingDatastoreTableErr(err) {
				count = 0
			} else {
				return nil, err
			}
		}

		out = append(out, map[string]interface{}{
			"model": modelType.Name,
			"count": count,
		})
	}

	return out, nil
}

func (s *GraphQLServer) GetProjectResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var projectID string
	if val, ok := p.Args["_id"].(string); ok {
		projectID = strings.TrimSpace(val)
	}
	// Empty _id means "current project" (matches GraphQL field description).
	if projectID == "" {
		v := p.Context.Value
		router, ok := v("router").(echo.Context)
		if !ok {
			return nil, errors.New("router context missing")
		}
		cache, err := s.GetApplicationCache(router)
		if err != nil {
			return nil, err
		}
		if cache.Project == nil || strings.TrimSpace(cache.Project.ID) == "" {
			return nil, errors.New(ae.MODEL_NAME_REQUIRED)
		}
		projectID = cache.Project.ID
	}
	project, err := s.SystemDriver.GetProject(p.Context, projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("project not found")
		}
		return nil, err
	}
	project.Schema = nil
	return project, nil
}

func (s *GraphQLServer) GetLoggedInUserFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapProjectsRead); err != nil {
		return nil, err
	}

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	userID := param.UserID

	user, err := s.SystemDriver.GetSystemUser(p.Context, userID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (s *GraphQLServer) ListAuditLogsFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	// if _id available then change the project in param
	if val, ok := p.Args["_id"].(string); ok && val != "" {
		param.ProjectID = val
	}

	param.ResolveParams = &p
	param.SystemCollectionName = "audit_logs"

	resp, err := s.SystemDriver.SearchAuditLogs(p.Context, param)
	if err != nil {
		return nil, err
	}

	return resp.Results, nil
}

func (s *GraphQLServer) ListWebHooksResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	param.ResolveParams = &p
	param.SystemCollectionName = "webhooks"

	res, err := s.SystemDriver.SearchWebHooks(p.Context, param)
	if err != nil {
		return nil, err
	}

	return res.Results, nil
}

func (s *GraphQLServer) ListModelsInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapSchemaRead); err != nil {
		return nil, err
	}

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	if project.Schema == nil {
		//return nil, errors.New(ae.SchemaIsNil)
		return []*models.ModelType{}, nil
	}

	var rawModelName string
	if val, ok := p.Args["model_name"].(string); ok {
		rawModelName = strings.TrimSpace(val)
	}

	if rawModelName != "" {
		singular := utility.SingularResourceName(rawModelName)
		var modelType *models.ModelType
		for _, model := range project.Schema.Models {
			if utility.ModelIDMatchesGraphQLField(model.Name, singular) || model.Name == rawModelName {
				modelType = model
				break
			}
		}

		if modelType == nil {
			return nil, ae.ModelTypeNotFound
		}

		modelType.Fields = models.DedupeFieldsByIdentifier(modelType.Fields)

		// search and add locals
		var locals pie.Strings
		for _, f := range modelType.Fields {
			if f.Validation != nil {
				if len(f.Validation.Locals) > 0 {
					locals = append(locals, f.Validation.Locals...)
				}

				/* if f.Validation.IsSystemRole {
					//f.Validation.FixedListElements = s.SystemRoles
				} */
			}
		}

		modelType.Locals = locals.Unique()

		if len(modelType.Connections) > 0 {
			modelType.HasConnections = true
		}

		return []*models.ModelType{modelType}, nil // resp is a list
	}

	for _, m := range project.Schema.Models {
		if m == nil {
			continue
		}
		m.Fields = models.DedupeFieldsByIdentifier(m.Fields)
		if len(m.Connections) > 0 {
			m.HasConnections = true
		}
	}

	return project.Schema.Models, nil
}

// ProjectSchemaRelationGraphResolverFn returns a JSON payload (edges, models, mermaid) for MCP / tooling.
// Schema is taken from the current application cache project (same scope as projectModelsInfo without model_name).
func (s *GraphQLServer) ProjectSchemaRelationGraphResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapSchemaRead); err != nil {
		return nil, err
	}

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}
	project := cache.Project
	if project == nil || project.Schema == nil {
		return map[string]interface{}{
			"project_id": projectIDOrEmpty(project),
			"models":     []string{},
			"edges":      []interface{}{},
			"mermaid":    "erDiagram\n  NO_SCHEMA {\n    string note \"no schema\"\n  }\n",
		}, nil
	}

	if val, ok := p.Args["_id"].(string); ok && strings.TrimSpace(val) != "" {
		if project.ID != strings.TrimSpace(val) {
			return nil, fmt.Errorf("%w :: relation graph is only available for the active project", ae.NotAllowed)
		}
	}

	return buildProjectRelationGraphPayload(project), nil
}

func projectIDOrEmpty(p *models.Project) string {
	if p == nil {
		return ""
	}
	return p.ID
}

func buildProjectRelationGraphPayload(project *models.Project) map[string]interface{} {
	modelSet := make(map[string]struct{})
	var edges []map[string]interface{}
	for _, m := range project.Schema.Models {
		if m == nil || m.Name == "" {
			continue
		}
		modelSet[m.Name] = struct{}{}
		for _, c := range m.Connections {
			if c == nil || c.Model == "" {
				continue
			}
			modelSet[c.Model] = struct{}{}
			edges = append(edges, map[string]interface{}{
				"from":     m.Name,
				"to":       c.Model,
				"relation": c.Relation,
				"type":     c.Type,
				"known_as": c.KnownAs,
			})
		}
	}
	models := make([]string, 0, len(modelSet))
	for name := range modelSet {
		models = append(models, name)
	}
	sort.Strings(models)

	mermaid := buildRelationGraphMermaid(models, edges)

	return map[string]interface{}{
		"project_id": project.ID,
		"models":     models,
		"edges":      edgesJSON(edges),
		"mermaid":    mermaid,
	}
}

func edgesJSON(edges []map[string]interface{}) []interface{} {
	out := make([]interface{}, len(edges))
	for i := range edges {
		out[i] = edges[i]
	}
	return out
}

func mermaidSafeID(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if s == "" {
		return "model"
	}
	return s
}

func mermaidNodeIDs(models []string) map[string]string {
	out := make(map[string]string)
	used := make(map[string]struct{})
	for _, name := range models {
		base := mermaidSafeID(name)
		id := base
		for n := 0; ; n++ {
			if _, ok := used[id]; !ok {
				used[id] = struct{}{}
				out[name] = id
				break
			}
			id = fmt.Sprintf("%s_%d", base, n)
		}
	}
	return out
}

func buildRelationGraphMermaid(models []string, edges []map[string]interface{}) string {
	idFor := mermaidNodeIDs(models)

	var sb strings.Builder
	sb.WriteString("erDiagram\n")

	connected := make(map[string]struct{})
	for _, e := range edges {
		from, _ := e["from"].(string)
		to, _ := e["to"].(string)
		if from == "" || to == "" || from == to {
			continue
		}
		fid, ok1 := idFor[from]
		tid, ok2 := idFor[to]
		if !ok1 || !ok2 {
			continue
		}
		connected[from] = struct{}{}
		connected[to] = struct{}{}

		rel, _ := e["relation"].(string)
		ka, _ := e["known_as"].(string)
		card := erDiagramCardinality(rel)
		label := erDiagramEdgeLabel(rel, ka)
		sb.WriteString(fmt.Sprintf("  %s %s %s : \"%s\"\n", fid, card, tid, label))
	}

	for _, name := range models {
		if _, ok := connected[name]; ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("  %s\n", idFor[name]))
	}

	if sb.String() == "erDiagram\n" {
		return "erDiagram\n  NO_CONNECTIONS {\n    string note \"no connections\"\n  }\n"
	}
	return sb.String()
}

// erDiagramCardinality maps Apito connection relation to Mermaid erDiagram crow's-foot line.
func erDiagramCardinality(rel string) string {
	switch strings.ToLower(strings.TrimSpace(rel)) {
	case "has_many":
		return "||--o{"
	case "has_one":
		return "||--||"
	case "many_to_many", "many-many", "many_many":
		return "}o--o{"
	default:
		return "||--o{"
	}
}

func erDiagramEdgeLabel(rel, knownAs string) string {
	rel = strings.TrimSpace(rel)
	knownAs = strings.TrimSpace(knownAs)
	var b strings.Builder
	if rel != "" {
		b.WriteString(rel)
	}
	if knownAs != "" {
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('(')
		b.WriteString(strings.ReplaceAll(knownAs, `"`, `'`))
		b.WriteByte(')')
	}
	s := b.String()
	if s == "" {
		return "relates"
	}
	return strings.ReplaceAll(s, `"`, `'`)
}

// resolveProjectModelFromSchema maps admin GraphQL `model` / `model_name` to ProjectSchema.Models[].Name.
// Naming V2 stores canonical snake_case (e.g. food_order). Using only utility.SingularResourceName("food_order")
// yields foodOrder and misses the schema row, leaving param.Model nil (breaks list/count and builds wrong Arango collection).
func resolveProjectModelFromSchema(schemaModels []*models.ModelType, modelArg string) *models.ModelType {
	raw := strings.TrimSpace(modelArg)
	if raw == "" {
		return nil
	}
	seen := make(map[string]struct{})
	add := func(keys *[]string, s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		*keys = append(*keys, s)
	}
	var candidates []string
	add(&candidates, raw)
	if c, err := utility.CanonicalizeModelName(raw); err == nil {
		add(&candidates, c)
	}
	if sn := utility.SingularResourceName(raw); sn != "" {
		add(&candidates, sn)
	}
	for _, k := range candidates {
		for _, field := range schemaModels {
			if field.Name == k {
				return field
			}
		}
	}
	return nil
}

func (s *GraphQLServer) ProjectModelInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)
	if err := requireAccessCapability(router, CapSchemaRead); err != nil {
		return nil, err
	}

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	doc := cache.Project

	var modelArg string
	if val, ok := p.Args["model_name"].(string); ok {
		modelArg = strings.TrimSpace(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	if doc.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	modelType := resolveProjectModelFromSchema(doc.Schema.Models, modelArg)

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	// search and add locals
	var locals pie.Strings
	for _, f := range modelType.Fields {
		if f.Validation != nil {
			if len(f.Validation.Locals) > 0 {
				locals = append(locals, f.Validation.Locals...)
			}

			/* if f.Validation.IsSystemRole {
				//f.Validation.FixedListElements = s.SystemRoles
			} */
		}
	}

	modelType.Locals = locals.Unique()

	if len(modelType.Connections) > 0 {
		modelType.HasConnections = true
	}

	return modelType, nil
}

func (s *GraphQLServer) SystemPlugins(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	// Load HashiCorp plugins from registry
	_hashiCorpPlugins, err := pluginService.LoadHashiCorpPluginRegistry(s.Cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load HashiCorp plugin registry: %w", err)
	}

	// Convert map to slice
	var _plugins []*protobuff.PluginDetails
	for _, plugin := range _hashiCorpPlugins {
		// try to find them in the loaded plugins cache
		if pluginCache, ok := s.HashiCorpPluginCache[plugin.Id]; ok {
			// overwrite the plugin configurations with the loaded plugin configurations
			_plugins = append(_plugins, pluginCache.PluginConfigurations)
		} else {
			_plugins = append(_plugins, plugin)
		}
	}

	return _plugins, nil
}

func (s *GraphQLServer) ProjectSpecificInstalledPlugins(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	// Load HashiCorp plugins from registry
	_hashiCorpPlugins, err := pluginService.LoadHashiCorpPluginRegistry(s.Cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to load HashiCorp plugin registry: %w", err)
	}

	// Convert map to slice
	var _plugins []*protobuff.PluginDetails
	for _, plugin := range _hashiCorpPlugins {
		for _, projectPlugin := range project.Plugins {
			if projectPlugin.ID == plugin.Id {
				plugin.LoadStatus = protobuff.PluginLoadStatus_PLUGIN_LOAD_STATUS_INSTALLED
				_plugins = append(_plugins, plugin)
				break
			}
		}
	}

	return _plugins, nil
}

// deprecated
/*func (s *GraphQLServer) ListModelsDataInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var modelName string
	if val, ok := p.Args["model"].(string); ok && val != "" {
		modelName = utility.SingularResourceName(val)
	} else {
		return nil, errors.New("Model Name is Necessary")
	}

	var modelType *models.ModelType
	for _, field := range s.ProjectRawSchemas.Models {
		if field.Name == modelName {
			modelType = field
		}
	}

	param := s.Param
	param.Model = modelType
	param.ResolveParams = &p

	return s.GetProjectDriver().GetAllPreviewDocumentsByModel(*param)
}*/

func (s *GraphQLServer) ListDetailedModelsDataProxyResolverFn(p graphql.ResolveParams) (interface{}, error) {
	return p, nil
}

func (s *GraphQLServer) ListDetailedModelsDataInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	// forward the proxy
	p.Args = p.Source.(graphql.ResolveParams).Args

	var modelArg string
	if val, ok := p.Args["model"].(string); ok && val != "" {
		modelArg = val
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	modelType := resolveProjectModelFromSchema(cache.Project.Schema.Models, modelArg)

	param := s.NewParam(cache.Param)
	param.Model = modelType
	param.ResolveParams = &p
	param.ProjectSchemaModels = cache.Project.Schema.Models

	param.IsSystemRequest = true

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	return driver.QueryMultiDocumentOfProject(cache.Ctx, param)
}

func (s *GraphQLServer) ListDetailedModelsDataCountResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	// forward the proxy
	p.Args = p.Source.(graphql.ResolveParams).Args

	var modelArg string
	if val, ok := p.Args["model"].(string); ok && val != "" {
		modelArg = val
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	modelType := resolveProjectModelFromSchema(cache.Project.Schema.Models, modelArg)

	param := s.NewParam(cache.Param)
	param.Model = modelType
	param.ResolveParams = &p
	param.ProjectSchemaModels = cache.Project.Schema.Models

	param.IsSystemRequest = true

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	return driver.CountMultiDocumentOfProject(cache.Ctx, param, false)
}

func (s *GraphQLServer) ListProjectTeams(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	return s.SystemDriver.GetTeamsMembers(p.Context, param.ProjectID)
}

func (s *GraphQLServer) GetTeamsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	return s.SystemDriver.GetTeams(p.Context, param.UserID)
}

func (s *GraphQLServer) GetOrganizationsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	return s.SystemDriver.GetOrganizations(p.Context, param.UserID)
}

func (s *GraphQLServer) SearchSystemUsersResolverFn(p graphql.ResolveParams) (interface{}, error) {
	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	param.ResolveParams = &p
	param.SystemCollectionName = "users"

	resp, err := s.SystemDriver.SearchSystemUsers(p.Context, param)
	if err != nil {
		return nil, err
	}

	return resp.Results, nil
}

func (s *GraphQLServer) GetPhotosAndCountInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {
	return p, nil
}

/*func (s *GraphQLServer) GetPhotosInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	_param, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := _param.Project

	if project.ActivatedPluginsIds == nil || project.ActivatedPluginsIds.Storage == "" {
		return nil, errors.New("no activated plugin found")
	}

	var pluginCache *models.PluginCache
	if val, ok := s.LocalPluginCache[project.ActivatedPluginsIds.Storage]; ok && val != nil {
		pluginCache = val
	}

	if pluginCache == nil {
		return nil, errors.New("media plugin is not loaded")
	}

	// 2. look up a symbol (an exported function or variable)
	// in this case, variable Greeter
	pluginLookUp, err := pluginCache.Plugin.Lookup(pluginCache.PluginConfigurations.ExportedVariable)
	if err != nil {
		return nil, err
	}

	var storagePlugin interfaces.StoragePluginInterface
	storagePlugin, ok := pluginLookUp.(interfaces.StoragePluginInterface)
	if !ok {
		return nil, errors.New(fmt.Sprintf(`%s plugin load failed`, pluginCache.PluginConfigurations.ID))
	}

	// inject project id
	envs := []*extensions.EnvVariables{
		{
			Key:   "PROJECT_ID",
			Value: project.ID,
		},
	}
	err = storagePlugin.Init(envs)
	if err != nil {
		return nil, err
	}

	// forward the proxy
	p.Args = p.Source.(graphql.ResolveParams).Args

	// 2. init the plugin
	files, err := storagePlugin.ListFiles(p.Context, p.Args)
	if err != nil {
		return nil, err
	}

	//return s.Pixabay(p)
	return files, err
}

func (s *GraphQLServer) CountPhotosInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	_param, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := _param.Project

	if project.ActivatedPluginsIds == nil || project.ActivatedPluginsIds.Storage == "" {
		return nil, errors.New("no activated plugin found")
	}

	var pluginCache *models.PluginCache
	if val, ok := s.LocalPluginCache[project.ActivatedPluginsIds.Storage]; ok && val != nil {
		pluginCache = val
	}

	if pluginCache == nil {
		return nil, errors.New("media plugin is not loaded")
	}

	// 2. look up a symbol (an exported function or variable)
	// in this case, variable Greeter
	pluginLookUp, err := pluginCache.Plugin.Lookup(pluginCache.PluginConfigurations.ExportedVariable)
	if err != nil {
		return nil, err
	}

	var storagePlugin interfaces.StoragePluginInterface
	storagePlugin, ok := pluginLookUp.(interfaces.StoragePluginInterface)
	if !ok {
		return nil, errors.New(fmt.Sprintf(`%s plugin load failed`, pluginCache.PluginConfigurations.ID))
	}

	// inject project id
	envs := []*extensions.EnvVariables{
		{
			Key:   "PROJECT_ID",
			Value: project.ID,
		},
	}
	err = storagePlugin.Init(envs)
	if err != nil {
		return nil, err
	}

	// forward the proxy
	p.Args = p.Source.(graphql.ResolveParams).Args

	// 2. init the plugin
	count, err := storagePlugin.CountFiles(p.Context, p.Args)
	if err != nil {
		return nil, err
	}

	/*
		_param, err := s.buildCommonSystemParam(router)
		if err != nil {
			return nil, err
		}

		// forward the proxy
		p.Args = p.Source.(graphql.ResolveParams).Args

		param := s.NewParam(_param)
		param.ResolveParams = &p
		//return s.Pixabay(p)
		return s.GraphQLExecutor.GetProjectDriver(ctx).CountMedias(p.Context, param.ProjectId, &p)

	//return s.Pixabay(p)
	return count, err
}
*/

func (s *GraphQLServer) ListSingleModelDataInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	param.ResolveParams = &p

	if val, ok := p.Args["_id"].(string); ok {
		param.DocumentID = val
	} else {
		return nil, errors.New("ID is not provided")
	}

	if val, ok := p.Args["revision"].(bool); ok {
		param.Revision = val
	}

	var modelArg string
	if val, ok := p.Args["model"].(string); ok && val != "" {
		modelArg = val
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	modelType := resolveProjectModelFromSchema(cache.Project.Schema.Models, modelArg)

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}
	param.Model = modelType

	if val, ok := p.Args["single_page_data"].(bool); ok {
		param.SinglePageData = val
	}

	// fetch document of all status
	p.Args["status"] = "all"

	param.IsSystemRequest = true

	// no need for these selections in case of system query
	selections := helper.FieldToSelectionBuilder(modelType.Fields)
	param.QuerySelectionSets = &selections

	// no need for pagination and sorting in case of single document fetch
	param.SkipPagination = true
	param.SkipSort = true

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	doc, err := driver.GetSingleProjectDocument(cache.Ctx, param)
	if err != nil {
		return nil, err
	}

	// return empty data if single post data
	if doc.ID == "" && param.SinglePageData {
		return &types.DefaultDocumentStructure{
			Key:      param.Model.SinglePageUUID,
			ID:       param.Model.SinglePageUUID,
			Type:     param.Model.Name,
			Data:     nil,
			ExpireAt: "",
		}, nil
	}

	/* // add the meta
	docWithMeta, err := s.SystemDriver.AddSystemUserMetaInfo(p.Context, doc)
	if err != nil {
		return nil, err
	}
	doc.Meta = docWithMeta.Meta */

	return doc, nil
}

func (s *GraphQLServer) ListDocumentRevisionInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	param.ResolveParams = &p
	param.DocumentID = p.Args["_id"].(string)

	var modelArg string
	if val, ok := p.Args["model"].(string); ok && val != "" {
		modelArg = val
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	if val, ok := p.Args["single_page_data"].(bool); ok {
		param.SinglePageData = val
	}

	/*	if param.Plan == "free" {
		return []*models.DocumentRevisionHistory{}, nil
	}*/

	modelType := resolveProjectModelFromSchema(cache.Project.Schema.Models, modelArg)

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	param.Model = modelType
	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	doc, err := driver.GetSingleProjectDocumentRevisions(cache.Ctx, param)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func (s *GraphQLServer) ListSingleModelHasManyResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	projectId := cache.Project.ID

	formModel := p.Args["from_model"].(string)

	toModel := p.Args["to_model"].(string)
	var modelType *models.ModelType
	for _, model := range cache.Project.Schema.Models {
		if model.Name == toModel {
			modelType = model
			break
		}
	}

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	id := p.Args["_id"].(string)

	param := &models.CommonSystemParams{
		DocumentID:    id,
		ProjectID:     projectId,
		ResolveParams: &p,
		Model:         modelType,
	}

	if val, ok := p.Args["known_as"].(string); ok && val != "" {
		param.KnownAs = val
	}

	driver, err := s.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return nil, err
	}

	result, err := driver.GetAllRelationDocumentsOfSingleDocument(cache.Ctx, formModel, param)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"data": result,
	}, nil
}

func (s *GraphQLServer) ProjectFunctionsInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

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
	if err := requireAccessCapability(router, CapFunctionsRead); err != nil {
		return nil, err
	}

	param, err := s.buildCommonSystemParam(router)
	if err != nil {
		return nil, err
	}

	res, err := s.SystemDriver.SearchFunctions(p.Context, param)
	if err != nil {
		return nil, err
	}
	enrichActiveRevisionHashes(p.Context, s.lifecycleStore(), cache.Project.ID, res.Results)
	return res.Results, nil
}

// enrichActiveRevisionHashes fills ActiveRevisionHash from the lifecycle store
// for functions that have an ActiveRevisionID. Missing store/revision leaves hash empty.
func enrichActiveRevisionHashes(ctx context.Context, store interfaces.FunctionLifecycleStore, projectID string, fns []*models.ApitoFunction) {
	if store == nil || projectID == "" || len(fns) == 0 {
		return
	}
	for _, fn := range fns {
		if fn == nil || fn.ActiveRevisionID == "" {
			continue
		}
		rev, err := store.GetRevision(ctx, projectID, fn.ActiveRevisionID)
		if err != nil || rev == nil {
			continue
		}
		fn.ActiveRevisionHash = rev.ArtifactHash
	}
}

func (s *GraphQLServer) ListExecutableFunctionsResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	var modelArg string
	if val, ok := p.Args["model_name"].(string); ok && val != "" {
		modelArg = val
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	// if schema not found then create
	if cache.Project.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	modelType := resolveProjectModelFromSchema(cache.Project.Schema.Models, modelArg)

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	cache.Param.Model = modelType

	var supportedFunctions []string

	for _, ct := range cache.Project.Schema.Functions {
		if ct.Request.Model == modelType.Name || ct.Request.Model == "JSON" {
			supportedFunctions = append(supportedFunctions, ct.Name)
		}
	}

	return map[string]interface{}{
		"functions": supportedFunctions,
	}, nil
}

func (s *GraphQLServer) LoadedFunctionProviderResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var _type protobuff.PluginType
	if val, ok := p.Args["type"].(protobuff.PluginType); ok {
		_type = val
	}

	var _list []string

	// Only add HashiCorp plugins (local plugins removed)
	for _, cache := range s.HashiCorpPluginCache {
		if cache.PluginConfigurations != nil {
			_list = append(_list, cache.PluginConfigurations.Id)
		}
	}

	return map[string]interface{}{
		"type":    _type,
		"plugins": _list,
	}, nil
}

func (s *GraphQLServer) ListAvailableFunctionsResolverFn(p graphql.ResolveParams) (interface{}, error) {
	// Local plugin cache removed - function provider lookup disabled
	// Use HashiCorp plugins instead
	fmt.Println("Function provider lookup disabled for local plugins")
	return map[string]interface{}{}, nil
}

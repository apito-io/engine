package functions

import (
	"context"
	"fmt"
	"strings"

	"github.com/apito-io/types"
	"github.com/tailor-platform/graphql"
)

// RegisterReadDataOps replaces stubbed read handlers with driver-backed implementations.
// Write / http / email / jwt / secrets stay unavailable until separately implemented.
func RegisterReadDataOps(g *EngineDataGateway, deps GatewayDeps, registry *InvocationRegistry) {
	if g == nil || deps == nil {
		return
	}
	if registry == nil {
		registry = GlobalInvocationRegistry
	}
	g.CheckCapability = func(ctx context.Context, call *DataGatewayCall) error {
		inv, err := registry.Get(call.InvocationID)
		if err != nil {
			return err
		}
		return RequireCapability(inv.Capabilities)(ctx, call)
	}
	g.Register("getList", handleGetList(deps, registry))
	g.Register("getSingleResource", handleGetSingle(deps, registry))
	g.Register("getMany", handleGetMany(deps, registry))
	g.Register("listAllPages", handleGetList(deps, registry)) // guest SDK paginates via getList
	g.Register("getRelationDocuments", func(ctx context.Context, call *DataGatewayCall) (*DataGatewayResponse, error) {
		_ = ctx
		return &DataGatewayResponse{OK: false, Error: `data op "getRelationDocuments" not wired to project driver`}, nil
	})
}

func handleGetList(deps GatewayDeps, registry *InvocationRegistry) DataOpHandler {
	return func(ctx context.Context, call *DataGatewayCall) (*DataGatewayResponse, error) {
		inv, err := registry.Get(call.InvocationID)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		modelArg, _ := call.Payload["model"].(string)
		modelArg = strings.TrimSpace(modelArg)
		if modelArg == "" {
			return &DataGatewayResponse{OK: false, Error: "model is required"}, nil
		}
		modelType, err := deps.ResolveModel(inv, modelArg)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		if modelType == nil {
			return &DataGatewayResponse{OK: false, Error: fmt.Sprintf("model %q not found", modelArg)}, nil
		}
		param := deps.NewParam(inv)
		param.Model = modelType
		param.IsSystemRequest = true
		if inv.Project != nil && inv.Project.Schema != nil {
			param.ProjectSchemaModels = inv.Project.Schema.Models
		}

		args := map[string]interface{}{}
		if limit, ok := asFloat(call.Payload["limit"]); ok {
			args["limit"] = int(limit)
		} else {
			args["limit"] = 50
		}
		if page, ok := asFloat(call.Payload["page"]); ok {
			args["page"] = int(page)
		} else {
			args["page"] = 1
		}
		if filters, ok := call.Payload["filters"].(map[string]interface{}); ok {
			args["where"] = filters
		}
		if sort, ok := call.Payload["sort"].(map[string]interface{}); ok {
			args["sort"] = sort
		}
		param.ResolveParams = &graphql.ResolveParams{Args: args}

		dbCtx := inv.DBCtx
		if dbCtx == nil {
			dbCtx = ctx
		}
		driver, err := deps.GetProjectDriver(dbCtx, inv)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		docs, err := driver.QueryMultiDocumentOfProject(dbCtx, param)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		total := len(docs)
		if n, err := driver.CountMultiDocumentOfProject(dbCtx, param, false); err == nil {
			total = n
		}
		return &DataGatewayResponse{
			OK: true,
			Data: map[string]interface{}{
				"results": docsToMaps(docs),
				"total":   total,
			},
		}, nil
	}
}

func handleGetSingle(deps GatewayDeps, registry *InvocationRegistry) DataOpHandler {
	return func(ctx context.Context, call *DataGatewayCall) (*DataGatewayResponse, error) {
		inv, err := registry.Get(call.InvocationID)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		modelArg, _ := call.Payload["model"].(string)
		id, _ := call.Payload["id"].(string)
		modelArg = strings.TrimSpace(modelArg)
		id = strings.TrimSpace(id)
		if modelArg == "" || id == "" {
			return &DataGatewayResponse{OK: false, Error: "model and id are required"}, nil
		}
		modelType, err := deps.ResolveModel(inv, modelArg)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		if modelType == nil {
			return &DataGatewayResponse{OK: false, Error: fmt.Sprintf("model %q not found", modelArg)}, nil
		}
		param := deps.NewParam(inv)
		param.Model = modelType
		param.DocumentID = id
		param.IsSystemRequest = true
		if inv.Project != nil && inv.Project.Schema != nil {
			param.ProjectSchemaModels = inv.Project.Schema.Models
		}
		dbCtx := inv.DBCtx
		if dbCtx == nil {
			dbCtx = ctx
		}
		driver, err := deps.GetProjectDriver(dbCtx, inv)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		doc, err := driver.GetSingleProjectDocument(dbCtx, param)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		return &DataGatewayResponse{OK: true, Data: docToMap(doc)}, nil
	}
}

func handleGetMany(deps GatewayDeps, registry *InvocationRegistry) DataOpHandler {
	return func(ctx context.Context, call *DataGatewayCall) (*DataGatewayResponse, error) {
		inv, err := registry.Get(call.InvocationID)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		modelArg, _ := call.Payload["model"].(string)
		modelArg = strings.TrimSpace(modelArg)
		ids := stringSlice(call.Payload["ids"])
		if modelArg == "" {
			return &DataGatewayResponse{OK: false, Error: "model is required"}, nil
		}
		modelType, err := deps.ResolveModel(inv, modelArg)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		if modelType == nil {
			return &DataGatewayResponse{OK: false, Error: fmt.Sprintf("model %q not found", modelArg)}, nil
		}
		param := deps.NewParam(inv)
		param.Model = modelType
		param.DocumentIDs = ids
		param.IsSystemRequest = true
		if inv.Project != nil && inv.Project.Schema != nil {
			param.ProjectSchemaModels = inv.Project.Schema.Models
		}
		args := map[string]interface{}{
			"limit": len(ids),
			"page":  1,
		}
		if len(ids) > 0 {
			args["where"] = map[string]interface{}{
				"_key": map[string]interface{}{"in": ids},
			}
		}
		param.ResolveParams = &graphql.ResolveParams{Args: args}
		dbCtx := inv.DBCtx
		if dbCtx == nil {
			dbCtx = ctx
		}
		driver, err := deps.GetProjectDriver(dbCtx, inv)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		docs, err := driver.QueryMultiDocumentOfProject(dbCtx, param)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		return &DataGatewayResponse{
			OK: true,
			Data: map[string]interface{}{
				"results": docsToMaps(docs),
				"total":   len(docs),
			},
		}, nil
	}
}

func docsToMaps(docs []*types.DefaultDocumentStructure) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(docs))
	for _, d := range docs {
		if m := docToMap(d); m != nil {
			out = append(out, m)
		}
	}
	return out
}

func docToMap(doc *types.DefaultDocumentStructure) map[string]interface{} {
	if doc == nil {
		return nil
	}
	id := doc.ID
	if id == "" {
		id = doc.Key
	}
	data := doc.Data
	if data == nil {
		data = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":   id,
		"data": data,
	}
}

func asFloat(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case int32:
		return float64(n), true
	default:
		return 0, false
	}
}

func stringSlice(v interface{}) []string {
	switch raw := v.(type) {
	case []string:
		return raw
	case []interface{}:
		out := make([]string, 0, len(raw))
		for _, item := range raw {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

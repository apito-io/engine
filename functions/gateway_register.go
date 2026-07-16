package functions

import (
	"context"
	"fmt"
)

// RegisterCoreDataOps registers the standard data op names on a gateway.
// Handlers must be supplied by the engine (project driver bound); this only
// installs stubs that return "not wired" so capability checks can be tested.
func RegisterCoreDataOps(g *EngineDataGateway, batch ProjectBatchExecutor) {
	if g == nil {
		return
	}
	stub := func(op string) DataOpHandler {
		return func(ctx context.Context, call *DataGatewayCall) (*DataGatewayResponse, error) {
			_ = ctx
			return &DataGatewayResponse{OK: false, Error: fmt.Sprintf("data op %q not wired to project driver", op)}, nil
		}
	}
	for _, op := range []string{
		"getSingleResource", "getMany", "getRelationDocuments", "getList", "listAllPages",
		"createNewResource", "updateResource", "deleteOne",
	} {
		g.Register(op, stub(op))
	}
	g.Register("transaction", func(ctx context.Context, call *DataGatewayCall) (*DataGatewayResponse, error) {
		if batch == nil {
			return &DataGatewayResponse{OK: false, Error: "ProjectBatchExecutor not configured"}, nil
		}
		opsRaw, _ := call.Payload["operations"].([]interface{})
		ops := make([]BatchOp, 0, len(opsRaw))
		for _, raw := range opsRaw {
			m, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			op := BatchOp{
				Op:    str(m["op"]),
				Model: str(m["model"]),
				ID:    str(m["id"]),
				Field: str(m["field"]),
			}
			if p, ok := m["payload"].(map[string]interface{}); ok {
				op.Payload = p
			}
			if c, ok := m["connect"].(map[string]interface{}); ok {
				op.Connect = c
			}
			if d, ok := m["disconnect"].(map[string]interface{}); ok {
				op.Disconnect = d
			}
			if by, ok := m["by"].(float64); ok {
				op.By = by
			}
			ops = append(ops, op)
		}
		if err := ValidateBatchOps(ops); err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		req := &BatchRequest{
			IdempotencyKey: str(call.Payload["idempotency_key"]),
			Operations:     ops,
		}
		res, err := batch.ExecuteBatch(ctx, call.ProjectID, call.TenantID, req)
		if err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
		return &DataGatewayResponse{OK: res.OK, Data: map[string]interface{}{
			"ids":    res.IDs,
			"replay": res.Replay,
			"error":  res.Error,
		}}, nil
	})
	g.Register("log", func(ctx context.Context, call *DataGatewayCall) (*DataGatewayResponse, error) {
		_ = ctx
		return &DataGatewayResponse{OK: true, Data: map[string]interface{}{"logged": true}}, nil
	})
	g.Register("http.fetch", stub("http.fetch"))
	g.Register("email.send", stub("email.send"))
	g.Register("jwt.sign", stub("jwt.sign"))
	g.Register("jwt.verify", stub("jwt.verify"))
	g.Register("secrets.get", stub("secrets.get"))
}

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}

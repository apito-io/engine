package functions

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// EngineDataGateway is a capability-aware data gateway backed by injectable handlers.
// The engine wires real project-driver operations; this package stays free of driver imports.
type EngineDataGateway struct {
	mu       sync.RWMutex
	handlers map[string]DataOpHandler
	// CheckCapability returns nil if the call is allowed.
	CheckCapability func(ctx context.Context, call *DataGatewayCall) error
	HostCallCount   map[string]int // invocationID → count
	MaxHostCalls    int
}

// DataOpHandler handles one gateway op.
type DataOpHandler func(ctx context.Context, call *DataGatewayCall) (*DataGatewayResponse, error)

// NewEngineDataGateway creates an empty gateway.
func NewEngineDataGateway() *EngineDataGateway {
	return &EngineDataGateway{
		handlers:      make(map[string]DataOpHandler),
		HostCallCount: make(map[string]int),
		MaxHostCalls:  200,
	}
}

// Register binds an op name to a handler.
func (g *EngineDataGateway) Register(op string, h DataOpHandler) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.handlers[op] = h
}

// Handle implements FunctionDataGateway.
func (g *EngineDataGateway) Handle(ctx context.Context, call *DataGatewayCall) (*DataGatewayResponse, error) {
	if call == nil {
		return &DataGatewayResponse{OK: false, Error: "nil call"}, nil
	}
	g.mu.Lock()
	g.HostCallCount[call.InvocationID]++
	n := g.HostCallCount[call.InvocationID]
	max := g.MaxHostCalls
	g.mu.Unlock()
	if max > 0 && n > max {
		return &DataGatewayResponse{OK: false, Error: "host call limit exceeded"}, nil
	}
	if g.CheckCapability != nil {
		if err := g.CheckCapability(ctx, call); err != nil {
			return &DataGatewayResponse{OK: false, Error: err.Error()}, nil
		}
	}
	g.mu.RLock()
	h, ok := g.handlers[call.Op]
	g.mu.RUnlock()
	if !ok {
		return &DataGatewayResponse{OK: false, Error: fmt.Sprintf("unsupported data op %q", call.Op)}, nil
	}
	return h(ctx, call)
}

// RequireCapability is a helper CheckCapability for data.read/data.write prefixes.
func RequireCapability(caps []string) func(context.Context, *DataGatewayCall) error {
	allowed := make(map[string]bool, len(caps))
	for _, c := range caps {
		allowed[c] = true
	}
	return func(_ context.Context, call *DataGatewayCall) error {
		model, _ := call.Payload["model"].(string)
		need := ""
		switch call.Op {
		case "getSingleResource", "getMany", "getRelationDocuments", "getList", "listAllPages":
			need = "data.read:" + model
			if allowed["data.read:*"] || allowed[need] || allowed["data.read"] {
				return nil
			}
		case "createNewResource", "updateResource", "deleteOne", "transaction", "inc":
			need = "data.write:" + model
			if allowed["data.write:*"] || allowed[need] || allowed["data.write"] {
				return nil
			}
		case "log":
			return nil
		case "http.fetch":
			if allowed["http"] {
				return nil
			}
			need = "http"
		case "secrets.get":
			if allowed["secrets"] {
				return nil
			}
			need = "secrets"
		case "jwt.sign", "jwt.verify":
			if allowed["jwt"] {
				return nil
			}
			need = "jwt"
		case "email.send":
			if allowed["email"] {
				return nil
			}
			need = "email"
		default:
			return fmt.Errorf("unknown op %q", call.Op)
		}
		if need == "" {
			return nil
		}
		// Also allow bare data.read / data.write without model when model empty
		if model == "" && (strings.HasPrefix(need, "data.read") && allowed["data.read"] ||
			strings.HasPrefix(need, "data.write") && allowed["data.write"]) {
			return nil
		}
		return fmt.Errorf("capability denied: need %s", need)
	}
}

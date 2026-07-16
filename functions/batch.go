package functions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
)

// MemoryBatchExecutor is an in-memory ProjectBatchExecutor for tests and drivers
// that have not yet implemented transactional batches. Production SQLite wiring
// replaces this with a driver-backed executor.
type MemoryBatchExecutor struct {
	mu    sync.Mutex
	ledger map[string]*BatchResult // idempotency key → result
	Apply func(ctx context.Context, projectID, tenantScope string, op BatchOp) (string, error)
}

// NewMemoryBatchExecutor creates a batch executor with an optional Apply hook.
func NewMemoryBatchExecutor() *MemoryBatchExecutor {
	return &MemoryBatchExecutor{ledger: make(map[string]*BatchResult)}
}

func (e *MemoryBatchExecutor) ExecuteBatch(ctx context.Context, projectID, tenantScope string, req *BatchRequest) (*BatchResult, error) {
	if req == nil {
		return &BatchResult{OK: false, Error: "nil batch request"}, nil
	}
	if req.IdempotencyKey == "" {
		return &BatchResult{OK: false, Error: "idempotency_key required"}, nil
	}
	key := projectID + "|" + tenantScope + "|" + req.IdempotencyKey

	e.mu.Lock()
	if prev, ok := e.ledger[key]; ok && prev.StatusCommitted() {
		cp := *prev
		cp.Replay = true
		e.mu.Unlock()
		return &cp, nil
	}
	e.mu.Unlock()

	reqHash := hashBatch(req)
	ids := make([]string, 0, len(req.Operations))
	for _, op := range req.Operations {
		if e.Apply != nil {
			id, err := e.Apply(ctx, projectID, tenantScope, op)
			if err != nil {
				fail := &BatchResult{OK: false, Error: err.Error()}
				e.mu.Lock()
				e.ledger[key] = fail
				e.mu.Unlock()
				return fail, nil
			}
			if id != "" {
				ids = append(ids, id)
			}
		} else {
			// Dry validation path
			if op.Model == "" || op.Op == "" {
				return &BatchResult{OK: false, Error: "op and model required"}, nil
			}
			if op.ID != "" {
				ids = append(ids, op.ID)
			}
		}
	}
	_ = reqHash
	ok := &BatchResult{OK: true, IDs: ids, Result: map[string]interface{}{"committed": true}}
	e.mu.Lock()
	e.ledger[key] = ok
	e.mu.Unlock()
	return ok, nil
}

func (r *BatchResult) StatusCommitted() bool { return r != nil && r.OK }

func hashBatch(req *BatchRequest) string {
	b, _ := json.Marshal(req.Operations)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ValidateBatchOps performs structural validation before execution.
func ValidateBatchOps(ops []BatchOp) error {
	if len(ops) == 0 {
		return fmt.Errorf("empty operations")
	}
	if len(ops) > 500 {
		return fmt.Errorf("too many operations (max 500)")
	}
	for i, op := range ops {
		switch op.Op {
		case "create", "update", "delete", "connect", "disconnect", "inc":
		default:
			return fmt.Errorf("operations[%d]: unsupported op %q", i, op.Op)
		}
		if op.Model == "" {
			return fmt.Errorf("operations[%d]: model required", i)
		}
		if (op.Op == "update" || op.Op == "delete" || op.Op == "inc") && op.ID == "" {
			return fmt.Errorf("operations[%d]: id required for %s", i, op.Op)
		}
		if op.Op == "inc" && op.Field == "" {
			return fmt.Errorf("operations[%d]: field required for inc", i)
		}
	}
	return nil
}

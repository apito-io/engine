package functions

import (
	"context"
	"time"

	"github.com/apito-io/engine/models"
)

// ABIVersion is the host/guest contract version for Apito Functions.
const ABIVersion = "1"

// InvocationEnvelope is the versioned request payload for a function invocation.
type InvocationEnvelope struct {
	Version      string                 `json:"version"`
	InvocationID string                 `json:"invocation_id"`
	ProjectID    string                 `json:"project_id"`
	TenantID     string                 `json:"tenant_id,omitempty"`
	Function     string                 `json:"function"`
	RevisionID   string                 `json:"revision_id,omitempty"`
	Runtime      string                 `json:"runtime"`
	UserID       string                 `json:"user_id,omitempty"`
	Role         string                 `json:"role,omitempty"`
	Principal    Principal              `json:"principal"`
	Capabilities []string               `json:"capabilities,omitempty"`
	Request      map[string]interface{} `json:"request"`
	DeadlineMs   int64                  `json:"deadline_ms"`
	Idempotency  string                 `json:"idempotency_key,omitempty"`
	ArtifactKey  string                 `json:"artifact_key,omitempty"`
	ArtifactHash string                 `json:"artifact_hash,omitempty"`
	Source       string                 `json:"source,omitempty"`
	Handler      string                 `json:"handler,omitempty"`
}

// Principal describes who the function runs as.
type Principal struct {
	Mode   string `json:"mode"` // caller | service
	UserID string `json:"user_id,omitempty"`
	Role   string `json:"role,omitempty"`
}

// InvocationResult is the versioned response from a function invocation.
type InvocationResult struct {
	Version      string                 `json:"version"`
	OK           bool                   `json:"ok"`
	Response     map[string]interface{} `json:"response,omitempty"`
	Error        string                 `json:"error,omitempty"`
	ErrorClass   string                 `json:"error_class,omitempty"` // timeout, oom, permission, egress, abort, …
	Logs         []LogEntry             `json:"logs,omitempty"`
	DurationMs   int64                  `json:"duration_ms,omitempty"`
	HostCalls    int                    `json:"host_calls,omitempty"`
	ResultDigest string                 `json:"result_digest,omitempty"`
}

// LogEntry is a bounded structured log line from a function.
type LogEntry struct {
	Level   string `json:"level"`
	Message string `json:"message"`
	At      string `json:"at,omitempty"`
}

// FunctionLimits captures per-invocation / per-function resource ceilings.
type FunctionLimits struct {
	MemoryBytes      int64
	Timeout          time.Duration
	MaxRequestBytes  int64
	MaxResponseBytes int64
	MaxLogBytes      int64
	MaxHostCalls     int
	MaxConcurrency   int
}

// DefaultCallableLimits returns conservative defaults for callable functions.
func DefaultCallableLimits() FunctionLimits {
	return FunctionLimits{
		MemoryBytes:      128 << 20, // 128 MiB (Deno-class)
		Timeout:          15 * time.Second,
		MaxRequestBytes:  1 << 20,  // 1 MiB
		MaxResponseBytes: 1 << 20,
		MaxLogBytes:      256 << 10, // 256 KiB
		MaxHostCalls:     200,
		MaxConcurrency:   4,
	}
}

// FunctionInvoker dispatches an invocation to the configured transport/runtime.
type FunctionInvoker interface {
	Invoke(ctx context.Context, env *InvocationEnvelope) (*InvocationResult, error)
}

// RuntimeProvider executes a function for a specific runtime (deno, wasm).
type RuntimeProvider interface {
	Name() string
	Execute(ctx context.Context, env *InvocationEnvelope, limits FunctionLimits) (*InvocationResult, error)
}

// FunctionTransport delivers invocations to workers (local or NATS).
type FunctionTransport interface {
	Request(ctx context.Context, subject string, payload []byte, timeout time.Duration) ([]byte, error)
	RespondHandler(ctx context.Context, subject string, queueGroup string, handler func(payload []byte) ([]byte, error)) (func() error, error)
	Close() error
}

// ArtifactStore retrieves immutable function artifacts.
type ArtifactStore interface {
	Put(ctx context.Context, key string, data []byte, hash string) error
	Get(ctx context.Context, key string) ([]byte, string, error)
	Delete(ctx context.Context, key string) error
}

// FunctionDataGateway fulfills host data/integration calls from a sandbox.
type FunctionDataGateway interface {
	Handle(ctx context.Context, call *DataGatewayCall) (*DataGatewayResponse, error)
}

// DataGatewayCall is a host-side data/integration request from a function.
type DataGatewayCall struct {
	InvocationID string                 `json:"invocation_id"`
	ProjectID    string                 `json:"project_id"`
	TenantID     string                 `json:"tenant_id,omitempty"`
	Op           string                 `json:"op"`
	Payload      map[string]interface{} `json:"payload,omitempty"`
}

// DataGatewayResponse is the host response for a data/integration call.
type DataGatewayResponse struct {
	OK    bool                   `json:"ok"`
	Data  map[string]interface{} `json:"data,omitempty"`
	Error string                 `json:"error,omitempty"`
}

// BatchOp is one declarative mutation inside an atomic transaction.
type BatchOp struct {
	Op       string                 `json:"op"` // create, update, delete, connect, disconnect, inc
	Model    string                 `json:"model"`
	ID       string                 `json:"id,omitempty"`
	Payload  map[string]interface{} `json:"payload,omitempty"`
	Connect  map[string]interface{} `json:"connect,omitempty"`
	Disconnect map[string]interface{} `json:"disconnect,omitempty"`
	Field    string                 `json:"field,omitempty"` // for inc
	By       float64                `json:"by,omitempty"`    // for inc
}

// BatchRequest is the declarative unit-of-work form for ProjectBatchExecutor.
type BatchRequest struct {
	IdempotencyKey string    `json:"idempotency_key"`
	Operations     []BatchOp `json:"operations"`
}

// BatchResult is the outcome of an atomic batch.
type BatchResult struct {
	OK     bool                     `json:"ok"`
	IDs    []string                 `json:"ids,omitempty"`
	Error  string                   `json:"error,omitempty"`
	Replay bool                     `json:"replay,omitempty"`
	Result map[string]interface{}   `json:"result,omitempty"`
}

// ProjectBatchExecutor runs declarative ops in one driver transaction (optional capability).
type ProjectBatchExecutor interface {
	ExecuteBatch(ctx context.Context, projectID, tenantScope string, req *BatchRequest) (*BatchResult, error)
}

// FunctionLimitsProvider supplies plan/tier limits (pro injects; open-core uses defaults).
type FunctionLimitsProvider interface {
	LimitsFor(ctx context.Context, projectID string, fn *models.ApitoFunction) (FunctionLimits, error)
}

// BuildEnvelope constructs an invocation envelope from a function definition and GraphQL/REST args.
func BuildEnvelope(fn *models.ApitoFunction, projectID, tenantID, userID, role string, request map[string]interface{}, invocationID string) *InvocationEnvelope {
	timeoutMs := int64(15000)
	handler := ""
	runtime := ""
	if fn != nil {
		runtime = fn.EffectiveRuntime()
		if fn.RuntimeConfig != nil {
			if fn.RuntimeConfig.TimeOut > 0 {
				timeoutMs = fn.RuntimeConfig.TimeOut * 1000
				if fn.RuntimeConfig.TimeOut > 1000 {
					// Already milliseconds
					timeoutMs = fn.RuntimeConfig.TimeOut
				}
			}
			handler = fn.RuntimeConfig.Handler
		}
	}
	env := &InvocationEnvelope{
		Version:      ABIVersion,
		InvocationID: invocationID,
		ProjectID:    projectID,
		TenantID:     tenantID,
		Function:     "",
		Runtime:      runtime,
		UserID:       userID,
		Role:         role,
		Principal: Principal{
			Mode:   "caller",
			UserID: userID,
			Role:   role,
		},
		Request:    request,
		DeadlineMs: timeoutMs,
		Handler:    handler,
	}
	if fn != nil {
		env.Function = fn.Name
		env.RevisionID = fn.ActiveRevisionID
		env.Capabilities = fn.Capabilities
		env.Source = fn.Source
		env.ArtifactKey = fn.BinaryURL
	}
	return env
}

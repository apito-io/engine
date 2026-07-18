package functions

import (
	"context"
	"fmt"
	"sync"

	"github.com/apito-io/engine/models"
)

// InvocationContext holds trusted host state for one function invocation.
// Guest code never supplies project/tenant/capabilities — only the registry does.
type InvocationContext struct {
	Envelope     *InvocationEnvelope
	DBCtx        context.Context
	Param        *models.CommonSystemParams
	Project      *models.Project
	Capabilities []string
	Gateway      FunctionDataGateway
}

// InvocationRegistry maps invocation IDs to trusted host context.
type InvocationRegistry struct {
	mu    sync.RWMutex
	items map[string]*InvocationContext
}

// GlobalInvocationRegistry is the process-wide registry used by Deno bridge + gateway.
var GlobalInvocationRegistry = NewInvocationRegistry()

// NewInvocationRegistry creates an empty registry.
func NewInvocationRegistry() *InvocationRegistry {
	return &InvocationRegistry{items: make(map[string]*InvocationContext)}
}

// Register stores context for an invocation.
func (r *InvocationRegistry) Register(inv *InvocationContext) error {
	if r == nil || inv == nil || inv.Envelope == nil || inv.Envelope.InvocationID == "" {
		return fmt.Errorf("invalid invocation context")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[inv.Envelope.InvocationID] = inv
	return nil
}

// Get returns a registered invocation context.
func (r *InvocationRegistry) Get(invocationID string) (*InvocationContext, error) {
	if r == nil || invocationID == "" {
		return nil, fmt.Errorf("unknown invocation")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	inv, ok := r.items[invocationID]
	if !ok || inv == nil {
		return nil, fmt.Errorf("unknown invocation %q", invocationID)
	}
	return inv, nil
}

// Unregister removes an invocation context.
func (r *InvocationRegistry) Unregister(invocationID string) {
	if r == nil || invocationID == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, invocationID)
}

package functions

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// RuntimeManager routes Apito Function invocations to the correct RuntimeProvider
// via a FunctionTransport. HashiCorp plugins remain outside this manager.
type RuntimeManager struct {
	providers map[string]RuntimeProvider
	transport FunctionTransport
	limits    FunctionLimitsProvider
	gateway   FunctionDataGateway
	artifacts ArtifactStore

	mu          sync.Mutex
	semaphores  map[string]chan struct{} // projectID → concurrency semaphore
	globalSem   chan struct{}
	defaultLim  FunctionLimits
}

// ManagerOption configures RuntimeManager.
type ManagerOption func(*RuntimeManager)

// WithTransport sets the function transport.
func WithTransport(t FunctionTransport) ManagerOption {
	return func(m *RuntimeManager) { m.transport = t }
}

// WithLimitsProvider sets the limits provider.
func WithLimitsProvider(p FunctionLimitsProvider) ManagerOption {
	return func(m *RuntimeManager) { m.limits = p }
}

// WithDataGateway sets the data gateway.
func WithDataGateway(g FunctionDataGateway) ManagerOption {
	return func(m *RuntimeManager) { m.gateway = g }
}

// WithArtifactStore sets the artifact store.
func WithArtifactStore(s ArtifactStore) ManagerOption {
	return func(m *RuntimeManager) { m.artifacts = s }
}

// WithGlobalConcurrency sets the process-wide concurrency semaphore.
func WithGlobalConcurrency(n int) ManagerOption {
	return func(m *RuntimeManager) {
		if n > 0 {
			m.globalSem = make(chan struct{}, n)
		}
	}
}

// NewRuntimeManager creates a manager with the given providers.
func NewRuntimeManager(providers []RuntimeProvider, opts ...ManagerOption) *RuntimeManager {
	m := &RuntimeManager{
		providers:  make(map[string]RuntimeProvider),
		semaphores: make(map[string]chan struct{}),
		defaultLim: DefaultCallableLimits(),
		globalSem:  make(chan struct{}, 16),
	}
	for _, p := range providers {
		if p != nil {
			m.providers[p.Name()] = p
		}
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// RegisterProvider adds or replaces a runtime provider.
func (m *RuntimeManager) RegisterProvider(p RuntimeProvider) {
	if p == nil {
		return
	}
	m.providers[p.Name()] = p
}

// DataGateway returns the configured data gateway (may be nil).
func (m *RuntimeManager) DataGateway() FunctionDataGateway { return m.gateway }

// Artifacts returns the artifact store (may be nil).
func (m *RuntimeManager) Artifacts() ArtifactStore { return m.artifacts }

// Invoke implements FunctionInvoker.
func (m *RuntimeManager) Invoke(ctx context.Context, env *InvocationEnvelope) (*InvocationResult, error) {
	if env == nil {
		return nil, fmt.Errorf("nil invocation envelope")
	}
	if env.Version == "" {
		env.Version = ABIVersion
	}
	if env.InvocationID == "" {
		env.InvocationID = utility.NewID()
	}

	limits := m.defaultLim
	if m.limits != nil {
		fn := &models.ApitoFunction{Name: env.Function}
		if env.Runtime != "" {
			fn.RuntimeConfig = &models.ApitoFunctionRuntimeConfig{Runtime: env.Runtime}
		}
		if l, err := m.limits.LimitsFor(ctx, env.ProjectID, fn); err == nil {
			limits = l
		}
	}
	if env.DeadlineMs > 0 {
		limits.Timeout = time.Duration(env.DeadlineMs) * time.Millisecond
	}

	// Admission control
	if m.globalSem != nil {
		select {
		case m.globalSem <- struct{}{}:
			defer func() { <-m.globalSem }()
		case <-ctx.Done():
			return &InvocationResult{Version: ABIVersion, OK: false, Error: "admission timeout", ErrorClass: "overload"}, ctx.Err()
		default:
			select {
			case m.globalSem <- struct{}{}:
				defer func() { <-m.globalSem }()
			case <-time.After(2 * time.Second):
				return &InvocationResult{Version: ABIVersion, OK: false, Error: "global concurrency limit", ErrorClass: "overload"}, nil
			case <-ctx.Done():
				return &InvocationResult{Version: ABIVersion, OK: false, Error: ctx.Err().Error(), ErrorClass: "cancelled"}, ctx.Err()
			}
		}
	}

	sem := m.projectSem(env.ProjectID, limits.MaxConcurrency)
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	case <-ctx.Done():
		return &InvocationResult{Version: ABIVersion, OK: false, Error: ctx.Err().Error(), ErrorClass: "cancelled"}, ctx.Err()
	}

	ctx, cancel := context.WithTimeout(ctx, limits.Timeout)
	defer cancel()

	provider, ok := m.providers[env.Runtime]
	if !ok {
		return &InvocationResult{
			Version:    ABIVersion,
			OK:         false,
			Error:      fmt.Sprintf("no runtime provider for %q", env.Runtime),
			ErrorClass: "runtime_mismatch",
		}, nil
	}

	start := time.Now()
	result, err := provider.Execute(ctx, env, limits)
	if result == nil {
		result = &InvocationResult{Version: ABIVersion, OK: false}
	}
	result.Version = ABIVersion
	result.DurationMs = time.Since(start).Milliseconds()
	if err != nil && result.Error == "" {
		result.Error = err.Error()
		if ctx.Err() != nil {
			result.ErrorClass = "timeout"
		}
	}
	return result, err
}

func (m *RuntimeManager) projectSem(projectID string, n int) chan struct{} {
	if n <= 0 {
		n = m.defaultLim.MaxConcurrency
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	sem, ok := m.semaphores[projectID]
	if !ok || cap(sem) != n {
		sem = make(chan struct{}, n)
		m.semaphores[projectID] = sem
	}
	return sem
}

// DefaultLimitsProvider always returns DefaultCallableLimits.
type DefaultLimitsProvider struct{}

func (DefaultLimitsProvider) LimitsFor(ctx context.Context, projectID string, fn *models.ApitoFunction) (FunctionLimits, error) {
	_ = ctx
	_ = projectID
	lim := DefaultCallableLimits()
	if fn != nil && fn.RuntimeConfig != nil {
		if fn.RuntimeConfig.Memory > 0 {
			// Memory is stored as MiB when small, bytes when large
			mem := fn.RuntimeConfig.Memory
			if mem < 1024 {
				mem = mem << 20
			}
			lim.MemoryBytes = mem
		}
		if fn.RuntimeConfig.TimeOut > 0 {
			t := fn.RuntimeConfig.TimeOut
			if t < 1000 {
				lim.Timeout = time.Duration(t) * time.Second
			} else {
				lim.Timeout = time.Duration(t) * time.Millisecond
			}
		}
		if fn.RuntimeConfig.Runtime == models.FunctionRuntimeWASM {
			if lim.MemoryBytes > 64<<20 {
				lim.MemoryBytes = 64 << 20
			}
			if lim.MemoryBytes == 128<<20 {
				lim.MemoryBytes = 32 << 20
			}
		}
	}
	return lim, nil
}

// HookLimitsProvider adapts Config.FunctionLimitsHook to FunctionLimitsProvider.
type HookLimitsProvider struct {
	Hook func(ctx context.Context, projectID string, fn *models.ApitoFunction) (memoryBytes int64, timeoutMs int64, maxConcurrency int, err error)
}

func (p HookLimitsProvider) LimitsFor(ctx context.Context, projectID string, fn *models.ApitoFunction) (FunctionLimits, error) {
	lim, err := DefaultLimitsProvider{}.LimitsFor(ctx, projectID, fn)
	if err != nil || p.Hook == nil {
		return lim, err
	}
	mem, timeoutMs, conc, err := p.Hook(ctx, projectID, fn)
	if err != nil {
		return lim, err
	}
	if mem > 0 {
		lim.MemoryBytes = mem
	}
	if timeoutMs > 0 {
		lim.Timeout = time.Duration(timeoutMs) * time.Millisecond
	}
	if conc > 0 {
		lim.MaxConcurrency = conc
	}
	return lim, nil
}

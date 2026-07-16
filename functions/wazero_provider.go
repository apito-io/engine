package functions

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

// WazeroRuntimeProvider executes prebuilt WASM artifacts with memory-page limits
// (via RuntimeConfig) and context cancellation. It does not claim native CPU fuel.
type WazeroRuntimeProvider struct {
	mu sync.Mutex
	// runtimes keyed by memory page class
	runtimes map[uint32]wazero.Runtime
	cache    map[string]wazero.CompiledModule // key: pages|artifactHash
}

// NewWazeroRuntimeProvider creates a provider.
func NewWazeroRuntimeProvider(ctx context.Context) *WazeroRuntimeProvider {
	_ = ctx
	return &WazeroRuntimeProvider{
		runtimes: make(map[uint32]wazero.Runtime),
		cache:    make(map[string]wazero.CompiledModule),
	}
}

func (p *WazeroRuntimeProvider) Name() string { return "wasm" }

func (p *WazeroRuntimeProvider) Close(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var first error
	for _, rt := range p.runtimes {
		if err := rt.Close(ctx); err != nil && first == nil {
			first = err
		}
	}
	p.runtimes = nil
	p.cache = nil
	return first
}

func (p *WazeroRuntimeProvider) Execute(ctx context.Context, env *InvocationEnvelope, limits FunctionLimits) (*InvocationResult, error) {
	var wasmBytes []byte
	if len(env.Source) >= 4 && env.Source[:4] == "\x00asm" {
		wasmBytes = []byte(env.Source)
	}
	if wasmBytes == nil {
		return &InvocationResult{
			Version: ABIVersion, OK: false,
			Error:      "wasm artifact not loaded into envelope (populate Source with wasm bytes or use ArtifactStore)",
			ErrorClass: "artifact_missing",
		}, nil
	}

	pages := uint32(512) // 32 MiB default
	if limits.MemoryBytes > 0 {
		pages = uint32((limits.MemoryBytes + 65535) / 65536)
		if pages < 1 {
			pages = 1
		}
		if pages > 1024 { // 64 MiB hard class
			pages = 1024
		}
	}

	rt, err := p.runtimeFor(ctx, pages)
	if err != nil {
		return &InvocationResult{Version: ABIVersion, OK: false, Error: err.Error(), ErrorClass: "abort"}, nil
	}

	cacheKey := fmt.Sprintf("%d|%s", pages, env.ArtifactHash)
	if env.ArtifactHash == "" {
		cacheKey = fmt.Sprintf("%d|%s|%s", pages, env.ArtifactKey, env.Function)
	}
	compiled, err := p.getOrCompile(ctx, rt, cacheKey, wasmBytes)
	if err != nil {
		return &InvocationResult{Version: ABIVersion, OK: false, Error: err.Error(), ErrorClass: "abi_rejection"}, nil
	}

	reqJSON, _ := json.Marshal(env.Request)
	cfg := wazero.NewModuleConfig().
		WithName(fmt.Sprintf("%s-%s", env.Function, env.InvocationID)).
		WithArgs("apito-fn", string(reqJSON))

	mod, err := rt.InstantiateModule(ctx, compiled, cfg)
	if err != nil {
		class := "abort"
		if ctx.Err() != nil {
			class = "timeout"
		}
		return &InvocationResult{Version: ABIVersion, OK: false, Error: err.Error(), ErrorClass: class}, nil
	}
	defer mod.Close(ctx)

	if fn := mod.ExportedFunction("execute"); fn != nil {
		if _, err := fn.Call(ctx); err != nil {
			class := "abort"
			if ctx.Err() != nil {
				class = "timeout"
			}
			return &InvocationResult{Version: ABIVersion, OK: false, Error: err.Error(), ErrorClass: class}, nil
		}
	} else if fn := mod.ExportedFunction("run"); fn != nil {
		if _, err := fn.Call(ctx); err != nil {
			class := "abort"
			if ctx.Err() != nil {
				class = "timeout"
			}
			return &InvocationResult{Version: ABIVersion, OK: false, Error: err.Error(), ErrorClass: class}, nil
		}
	}

	return &InvocationResult{
		Version: ABIVersion, OK: true,
		Response: map[string]interface{}{
			"ok":       true,
			"runtime":  "wasm",
			"function": env.Function,
		},
	}, nil
}

func (p *WazeroRuntimeProvider) runtimeFor(ctx context.Context, pages uint32) (wazero.Runtime, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if rt, ok := p.runtimes[pages]; ok {
		return rt, nil
	}
	cfg := wazero.NewRuntimeConfig().
		WithMemoryLimitPages(pages).
		WithCloseOnContextDone(true)
	rt := wazero.NewRuntimeWithConfig(ctx, cfg)
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)
	p.runtimes[pages] = rt
	return rt, nil
}

func (p *WazeroRuntimeProvider) getOrCompile(ctx context.Context, rt wazero.Runtime, key string, wasm []byte) (wazero.CompiledModule, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if c, ok := p.cache[key]; ok {
		return c, nil
	}
	compiled, err := rt.CompileModule(ctx, wasm)
	if err != nil {
		return nil, err
	}
	p.cache[key] = compiled
	return compiled, nil
}

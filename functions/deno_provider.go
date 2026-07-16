package functions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// DenoRuntimeProvider executes TypeScript/JavaScript via a pinned Deno binary.
// When Deno is not installed, Execute returns runtime_unavailable (use StubRuntimeProvider in tests).
type DenoRuntimeProvider struct {
	DenoPath string
	WorkDir  string
}

func (p *DenoRuntimeProvider) Name() string { return "deno" }

func (p *DenoRuntimeProvider) Execute(ctx context.Context, env *InvocationEnvelope, limits FunctionLimits) (*InvocationResult, error) {
	deno := p.DenoPath
	if deno == "" {
		deno = "deno"
	}
	if _, err := exec.LookPath(deno); err != nil {
		return &InvocationResult{
			Version:    ABIVersion,
			OK:         false,
			Error:      "deno binary not found; install pinned Deno for Apito Functions or use the compose worker image",
			ErrorClass: "runtime_unavailable",
		}, nil
	}

	root := p.WorkDir
	if root == "" {
		root = os.TempDir()
	}
	dir, err := os.MkdirTemp(root, "apito-fn-deno-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	source := env.Source
	if source == "" {
		source = `export default async function (req: Record<string, unknown>) { return { ok: true, echo: req }; }`
	}
	userPath := filepath.Join(dir, "user.ts")
	if err := os.WriteFile(userPath, []byte(source), 0o600); err != nil {
		return nil, err
	}
	reqPath := filepath.Join(dir, "request.json")
	reqBytes, _ := json.Marshal(env.Request)
	if err := os.WriteFile(reqPath, reqBytes, 0o600); err != nil {
		return nil, err
	}

	handler := env.Handler
	if handler == "" {
		handler = "default"
	}
	runner := fmt.Sprintf(`
import * as mod from "./user.ts";
const req = JSON.parse(await Deno.readTextFile(Deno.args[0]));
const fn = (mod as Record<string, unknown>)[%q] ?? (mod as { default?: unknown }).default;
if (typeof fn !== "function") {
  throw new Error("function handler %q not found");
}
const out = await (fn as (r: unknown) => Promise<unknown> | unknown)(req);
console.log(JSON.stringify(out ?? {}));
`, handler, handler)
	runnerPath := filepath.Join(dir, "runner.ts")
	if err := os.WriteFile(runnerPath, []byte(runner), 0o600); err != nil {
		return nil, err
	}

	args := []string{"run", "--quiet", "--no-prompt", "--allow-read=" + dir, runnerPath, reqPath}
	if hasCapability(env.Capabilities, "http") {
		args = []string{"run", "--quiet", "--no-prompt", "--allow-read=" + dir, "--allow-net", runnerPath, reqPath}
	}

	cmd := exec.CommandContext(ctx, deno, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err = cmd.Run()
	dur := time.Since(start).Milliseconds()
	_ = limits

	if ctx.Err() != nil {
		return &InvocationResult{
			Version: ABIVersion, OK: false, Error: "deadline exceeded", ErrorClass: "timeout", DurationMs: dur,
			Logs: []LogEntry{{Level: "error", Message: stderr.String()}},
		}, nil
	}
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return &InvocationResult{
			Version: ABIVersion, OK: false, Error: msg, ErrorClass: "abort", DurationMs: dur,
			Logs: []LogEntry{{Level: "error", Message: stderr.String()}},
		}, nil
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return &InvocationResult{
			Version: ABIVersion, OK: false,
			Error:      fmt.Sprintf("invalid function JSON response: %v", err),
			ErrorClass: "abort", DurationMs: dur,
			Logs: []LogEntry{{Level: "error", Message: stdout.String()}},
		}, nil
	}
	return &InvocationResult{Version: ABIVersion, OK: true, Response: resp, DurationMs: dur}, nil
}

func hasCapability(caps []string, name string) bool {
	for _, c := range caps {
		if c == name || strings.HasPrefix(c, name+":") {
			return true
		}
	}
	return false
}

// StubRuntimeProvider is a test/dev provider that echoes the request (no Deno binary required).
type StubRuntimeProvider struct {
	RuntimeName string
}

func (p *StubRuntimeProvider) Name() string {
	if p.RuntimeName == "" {
		return "deno"
	}
	return p.RuntimeName
}

func (p *StubRuntimeProvider) Execute(ctx context.Context, env *InvocationEnvelope, limits FunctionLimits) (*InvocationResult, error) {
	if env.Source != "" && strings.Contains(env.Source, "__FAIL__") {
		return &InvocationResult{Version: ABIVersion, OK: false, Error: "stub forced failure", ErrorClass: "abort"}, nil
	}
	if env.Source != "" && strings.Contains(env.Source, "__TIMEOUT__") {
		select {
		case <-ctx.Done():
			return &InvocationResult{Version: ABIVersion, OK: false, Error: "deadline exceeded", ErrorClass: "timeout"}, nil
		case <-time.After(limits.Timeout + time.Second):
		}
	}
	resp := map[string]interface{}{
		"ok":       true,
		"function": env.Function,
		"runtime":  p.Name(),
		"echo":     env.Request,
	}
	return &InvocationResult{Version: ABIVersion, OK: true, Response: resp}, nil
}

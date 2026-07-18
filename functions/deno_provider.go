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
	Gateway  FunctionDataGateway
	Registry *InvocationRegistry
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

	sdkPath := filepath.Join(dir, "apito_functions.js")
	if err := os.WriteFile(sdkPath, []byte(EmbeddedApitoFunctionsSDK), 0o600); err != nil {
		return nil, err
	}
	importMap := map[string]interface{}{
		"imports": map[string]string{
			"@apito-io/functions": "./apito_functions.js",
		},
	}
	importMapBytes, _ := json.Marshal(importMap)
	importMapPath := filepath.Join(dir, "import_map.json")
	if err := os.WriteFile(importMapPath, importMapBytes, 0o600); err != nil {
		return nil, err
	}

	resultPath := filepath.Join(dir, "result.json")
	handler := env.Handler
	if handler == "" {
		handler = "default"
	}

	var bridge *GatewayBridge
	registry := p.Registry
	if registry == nil {
		registry = GlobalInvocationRegistry
	}
	if p.Gateway != nil {
		inv, err := registry.Get(env.InvocationID)
		if err == nil && inv != nil {
			if inv.Gateway == nil {
				inv.Gateway = p.Gateway
			}
			bridge, err = StartGatewayBridge(ctx, p.Gateway, inv)
			if err != nil {
				return nil, fmt.Errorf("start gateway bridge: %w", err)
			}
			defer bridge.Close()
		}
	}

	shimPath := filepath.Join(dir, "shim.ts")
	// Avoid nested backticks: this string is emitted as a Go raw string.
	shim := "" +
		"import { ensureGlobalApito, createApitoFunctions } from \"@apito-io/functions\";\n" +
		"\n" +
		"const gatewayURL = Deno.env.get(\"APITO_GATEWAY_URL\") ?? \"\";\n" +
		"const gatewayToken = Deno.env.get(\"APITO_GATEWAY_TOKEN\") ?? \"\";\n" +
		"\n" +
		"if (gatewayURL && gatewayToken) {\n" +
		"  const transport = async (op: string, payload: Record<string, unknown>) => {\n" +
		"    const res = await fetch(gatewayURL + \"/call\", {\n" +
		"      method: \"POST\",\n" +
		"      headers: {\n" +
		"        \"Content-Type\": \"application/json\",\n" +
		"        Authorization: \"Bearer \" + gatewayToken,\n" +
		"      },\n" +
		"      body: JSON.stringify({ op, payload }),\n" +
		"    });\n" +
		"    const body = await res.json();\n" +
		"    if (!body?.ok) {\n" +
		"      throw new Error(body?.error || (\"gateway op \" + op + \" failed\"));\n" +
		"    }\n" +
		"    if (op === \"getList\" || op === \"listAllPages\" || op === \"getMany\") {\n" +
		"      return body.data ?? { results: [], total: 0 };\n" +
		"    }\n" +
		"    if (op === \"getSingleResource\") {\n" +
		"      return body.data ?? {};\n" +
		"    }\n" +
		"    return body.data ?? {};\n" +
		"  };\n" +
		"  ensureGlobalApito(transport);\n" +
		"} else {\n" +
		"  (globalThis as { apito?: unknown }).apito = createApitoFunctions(async (op) => {\n" +
		"    throw new Error(\"apito functions gateway unavailable for op \" + op);\n" +
		"  });\n" +
		"}\n"
	if err := os.WriteFile(shimPath, []byte(shim), 0o600); err != nil {
		return nil, err
	}

	runner := fmt.Sprintf(`
import "./shim.ts";
import * as mod from "./user.ts";
const req = JSON.parse(await Deno.readTextFile(Deno.args[0]));
const resultPath = Deno.args[1];
const logs: Array<{ level: string; message: string }> = [];
const origLog = console.log;
const origWarn = console.warn;
const origError = console.error;
console.log = (...args: unknown[]) => { logs.push({ level: "info", message: args.map(String).join(" ") }); origLog(...args); };
console.warn = (...args: unknown[]) => { logs.push({ level: "warn", message: args.map(String).join(" ") }); origWarn(...args); };
console.error = (...args: unknown[]) => { logs.push({ level: "error", message: args.map(String).join(" ") }); origError(...args); };
try {
  const handlerName = %q;
  const fn = (mod as Record<string, unknown>)[handlerName] ?? (mod as { default?: unknown }).default;
  if (typeof fn !== "function") {
    throw new Error("function handler " + handlerName + " not found");
  }
  const out = await (fn as (r: unknown) => Promise<unknown> | unknown)(req);
  await Deno.writeTextFile(resultPath, JSON.stringify({ ok: true, response: out ?? {}, logs }));
} catch (e) {
  const message = e instanceof Error ? e.message : String(e);
  await Deno.writeTextFile(resultPath, JSON.stringify({ ok: false, error: message, logs }));
  throw e;
}
`, handler)
	runnerPath := filepath.Join(dir, "runner.ts")
	if err := os.WriteFile(runnerPath, []byte(runner), 0o600); err != nil {
		return nil, err
	}

	allowNet := ""
	if bridge != nil {
		hostPort := strings.TrimPrefix(bridge.URL, "http://")
		allowNet = "--allow-net=" + hostPort
	} else if hasCapability(env.Capabilities, "http") {
		allowNet = "--allow-net"
	}

	args := []string{
		"run", "--quiet", "--no-prompt",
		"--import-map=" + importMapPath,
		"--allow-read=" + dir,
		"--allow-write=" + dir,
		"--allow-env=APITO_GATEWAY_URL,APITO_GATEWAY_TOKEN",
	}
	if allowNet != "" {
		args = append(args, allowNet)
	}
	args = append(args, runnerPath, reqPath, resultPath)

	cmd := exec.CommandContext(ctx, deno, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if bridge != nil {
		cmd.Env = append(os.Environ(),
			"APITO_GATEWAY_URL="+bridge.URL,
			"APITO_GATEWAY_TOKEN="+bridge.Token,
		)
	}

	start := time.Now()
	err = cmd.Run()
	dur := time.Since(start).Milliseconds()
	_ = limits

	resultBytes, readErr := os.ReadFile(resultPath)
	if readErr == nil && len(resultBytes) > 0 {
		var fileRes struct {
			OK       bool                   `json:"ok"`
			Response map[string]interface{} `json:"response"`
			Error    string                 `json:"error"`
			Logs     []LogEntry             `json:"logs"`
		}
		if json.Unmarshal(resultBytes, &fileRes) == nil {
			if ctx.Err() != nil {
				return &InvocationResult{
					Version: ABIVersion, OK: false, Error: "deadline exceeded", ErrorClass: "timeout", DurationMs: dur,
					Logs: fileRes.Logs,
				}, nil
			}
			if !fileRes.OK {
				msg := fileRes.Error
				if msg == "" {
					msg = strings.TrimSpace(stderr.String())
				}
				return &InvocationResult{
					Version: ABIVersion, OK: false, Error: msg, ErrorClass: "abort", DurationMs: dur,
					Logs: fileRes.Logs,
				}, nil
			}
			return &InvocationResult{
				Version: ABIVersion, OK: true, Response: fileRes.Response, DurationMs: dur,
				Logs: fileRes.Logs,
			}, nil
		}
	}

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

	// Fallback: legacy stdout JSON
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

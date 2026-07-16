# open-core — Handoff

## Branch
- Tracks engine `udbhabon` via go.mod replace in monorepo

## Done
- **Apito Functions foundation (2026-07-16):** `models/apito_function.go` + `function_lifecycle.go`; `functions/*` runtime stack; resolver routing by `EffectiveRuntime()`; `/function` constant-time secret verify; `docker-compose.functions.yml`. Tests: `functions/`, `resolver/` green. Uncommitted.

## Broken / watch
- Data gateway ops register stubs — not yet calling project DB driver
- Deno provider uses process spawn when `deno` on PATH; stub when missing
- Event dispatcher is in-memory only

## Next
- Driver-backed gateway + batch; console codegen hooks for lifecycle mutations
- Do not remove HashiCorp `hc-*` path until Rosna functions validated

## Do not touch
- Import `open_driver` / `pro_driver` / `plugin_system` / `nats_system` from open-core
- Generated GraphQL in console — edit `.ts` documents + codegen

## Last Updated
2026-07-16

# open-core — AI Changelog

Not git history — the *reasoning* behind changes. Newest on top.
Format per entry: date, **Changed**, **Why**, **Affected**.

---
## 2026-07-16 — v1.7.12 Deno in OSS Docker image
- **Changed:** Runtime stage `apk add deno`; CHANGELOG `[1.7.12]`.
- **Why:** Parity with Studio/engine images so self-host OSS can run Logic `runtime: deno`.
- **Affected:** `Dockerfile`, `CHANGELOG.md`. Tag when releasing: `v1.7.12`.

## 2026-07-16 — Apito Functions Platform foundation
- **Changed:** New `functions/` package (RuntimeManager, Deno/wazero providers, transport, artifact store, lifecycle, data gateway, batch/idempotency contracts, events); extended `ApitoFunction` model + lifecycle types; public schema merge/invoke routes deno/wasm; callable auth + REST secret verify; GraphQL server init wiring.
- **Why:** Enable Supabase-like TS functions with tenant-safe host SDK while preserving HashiCorp system plugins and fixing prior SQL/routing/auth blockers.
- **Affected:** `functions/*`, `models/apito_function.go`, `models/function_lifecycle.go`, `resolver/public_schema_function.go`, `resolver/public_schema_builder_build.go`, `resolver/function_runtime_init.go`, `controller/graph_controller.go`, `controller/callable_auth.go` (or `functions/callable_auth.go`), `docker-compose.functions.yml`.

## 2026-07-13 — v1.7.11
- **Changed:** `ModelIsSaaSTenantControlPlaneModel`, hook takes `*[]*PublicSchemaModelFilter`, skip tenant model in `collectFilteredModelsForPublicSchema`, mutation meta nil guards.
- **Why:** Engine pro SaaS tenant delete/catalog journey; studio clone builds against tagged open-core, not monorepo replace.
- **Affected:** `models/project_tenant_model.go`, `models/config.go`, `controller/public_schema_builder*.go`, `executor/solver.go`, `resolver/public_schema_mutation.go`.

## 2026-07-06
- **Changed:** Bootstrapped knowledge system for this repo.
- **Why:** Cross-LLM durable knowledge + working memory.
- **Affected:** this repo only.

Last Updated: 2026-07-16

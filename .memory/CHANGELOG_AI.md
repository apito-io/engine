# open-core — AI Changelog

Not git history — the *reasoning* behind changes. Newest on top.
Format per entry: date, **Changed**, **Why**, **Affected**.

---

## 2026-07-22 — v1.8.3 recursive SubFieldInfo + empty nested filters

- **Changed:** Recursive GraphQL `SubFieldInfo`; `_empty` placeholder for empty nested where filters.
- **Why:** CLI/MCP sync missed `exam.routine.details.*`; empty nested groups aborted public schema.
- **Affected:** `schemas/objects/object_models.go`, `search_filter_arg.go`, tests. Tagged **v1.8.3**.

---

## 2026-07-21 — apt_ fail-closed canonical project scope
- **Changed:** `applyAccessTokenScope` shared on secured + `X-Use-Cookies: false`;
  only `X-Apito-Project-Id` (no aliases); access_token claims never implicit
  `ProjectIDs[0]`; listProjects grant-filtered + `projects.read`;
  create/list/revoke/rotate console-session only; capability gates for secured
  GraphQL data, REST/files, schema/role/plugin/function/tenant ops.
- **Why:** Fresh multi-project automation contract — fail closed, one header forever.
- **Affected:** `services/token.go`, `access_policy.go`, `access_token_service.go`,
  `utility/claims_set.go`, `resolver/system_*.go`, `function_lifecycle_resolvers.go`,
  `controller/token_controller.go`, tests. Uncommitted.

## 2026-07-20→21 — Unified apt_ access tokens + session project override
- **Changed:** `AccessTokenRecord` on SystemUser; `authz` registry; mint/list/revoke/rotate;
  middleware `apt_` + `TOKEN_FORMAT_RETIRED`; project/tenant policy; REST
  `/system/access-tokens`. Cookie path: `applySessionProjectOverride` for
  `X-Apito-Project-Id` when user still administers target project (Console
  multi-project tenant pickers).
- **Why:** One revocable automation credential; Access Token UI must fetch
  tenants for non-current SaaS projects without switching active project cookie.
- **Affected:** `models/access_token.go`, `authz/*`, `services/access_token_service.go`,
  `access_policy.go`, `token.go`, `controller/token_controller.go`, `router/router.go`,
  `resolver/graphql_server.go` (GetApplicationCache gates). Uncommitted.

## 2026-07-20 — v1.7.14 active_revision_hash for function sync
- **Changed:** Non-persisted `ActiveRevisionHash` on ApitoFunction; GraphQL
  `active_revision_hash`; `projectFunctionsInfo` enriches from active revision
  artifact hash. CHANGELOG `[1.7.14]`.
- **Why:** CLI `sync --type functions` live-parity / deploy diffs need dest hash
  vs `sha256(source)` without N+1 revision lists.
- **Affected:** `models/apito_function.go`, `schemas/objects/object_models.go`,
  `resolver/system_query.go`, tests. Tag: `v1.7.14`.

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

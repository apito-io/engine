# open-core changelog

Notable changes to **open-core** only. Release tags use `v1.x` (for example `v1.5.4`, `v1.6.0`).

## [1.7.12] — 2026-07-16

### Packaging

- **Dockerfile** — Alpine runtime stage installs `deno` for Apito Logic functions (`runtime: deno`, in-process LocalTransport). No API / GraphQL contract changes.

### Apito Functions (foundation)

- **`functions/`** — RuntimeManager, Deno/wazero providers, LocalTransport, callable auth, data gateway/batch stubs, lifecycle models.
- **GraphQL** — `upsertFunctionToProject` + `source`/`trigger_type`; public schema invoke routing for deno/wasm; `FunctionRuntimeManager` init.

### Fixes

- **`modelDocumentCounts`** — missing physical tables (`no such table` / does not exist) return `count: 0` instead of failing the sidebar query.

## [1.7.11] — 2026-07-13

### API / behaviour

- **`ModelIsSaaSTenantControlPlaneModel`** — classifies the reserved SaaS tenant model (`system_tenant_model` / `is_tenant_model` ext keys) so public schema collection skips it; catalog lifecycle stays on system GraphQL.
- **`AdjustPublicSchemaForRequestHook`** — last parameter is now `*[]*PublicSchemaModelFilter` so pro hooks can remove models in place (breaking for hook implementors).
- **Mutation meta nil-safety** — public update/delete `own` scope and meta stamping guard nil `doc.Meta` / `CreatedBy` / `LastModifiedBy`.

## [1.6.0] — 2026-04-20

### Summary

Large release after `v1.5.4`: canonical model naming (naming schema v2), OpenTelemetry support, SaaS database routing fixes, and ArangoDB driver refactors. This release spans a wide set of packages under this tree.

### Highlights

- **Model naming v2** — `utility/apito_naming.go`, `CanonicalizeModelName`, English title helpers, `naming_vectors.json`, migration helpers (`naming_migration.go`), and resolver/schema builder updates so stored model ids (e.g. `food_order`) stay consistent with GraphQL and the system schema.
- **System resolvers** — `resolveProjectModelFromSchema` in `resolver/system_query.go` so `model` / `model_name` accept canonical snake_case as well as legacy singular camelCase (fixes `getModelData` when the schema uses `food_order`).
- **Telemetry** — `telemetry/` package: OTEL wiring, Echo middleware, DB decorator, pool observer.
- **SaaS and databases** — Shared-database SaaS fixes, per-tenant database flag and connection behaviour; driver factory and scoped credential updates across SQL, Mongo, bbolt, cache, KV, and queue layers.
- **Auth and tokens** — JWT, Branka, project token manager, and token service updates; login path fixes (commit `417d0c1`).
- **ArangoDB** — Refactor and removal work on the ArangoDB driver path (commit `1c73a0e`).

### Commits (newest first)

| Commit     | Subject |
|-----------|---------|
| `1c73a0e` | motamoti stable on the refractor of removal of arangodb |
| `94f060e` | saas for shared db fixed |
| `9ac1af6` | saas fixed |
| `6c130b9` | good so far |
| `b82c121` | per tenant db fix flag added |
| `417d0c1` | open-core login function fixed and few bug fixes |

## Earlier releases

Use `git tag -l 'v1.*'` and `git log` for history before `v1.6.0`.

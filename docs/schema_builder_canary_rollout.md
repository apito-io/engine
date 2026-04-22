# Public schema builder — canary rollout

Changes touch `publicSchemaBuilder`, compiled pre-connection cache, GraphQL context keys, and resolvers.

## Order

1. Deploy with defaults: `ENABLE_COMPILED_SCHEMA_CACHE=false`, `SCHEMA_BUILD_TELEMETRY=true`, `MAX_MODELS_PER_PROJECT=0` (unlimited).
2. Watch OTel spans `publicSchemaBuilder` (duration, `schema_cache_hit` when cache enabled).
3. Enable `ENABLE_COMPILED_SCHEMA_CACHE=true` in staging; mirror a slice of production traffic for 24h.
4. Compare p50/p95/p99 GraphQL latency and error rates vs baseline.
5. Roll out to production; keep `MAX_MODELS_PER_PROJECT` unset until abuse is observed, then set (e.g. 500).

## Rollback

- Set `ENABLE_COMPILED_SCHEMA_CACHE=false`.
- Revert commit if resolver/context regressions appear.

## Flags (see `open-core/models/config.go`)

| Env | Purpose |
|-----|---------|
| `ENABLE_COMPILED_SCHEMA_CACHE` | LRU cache of pre-connection GraphQL maps |
| `SCHEMA_BUILD_TELEMETRY` | OTel span around `publicSchemaBuilder` |
| `SCHEMA_BUILD_METRICS` | OTel `schema_build_total` + `schema_build_duration_seconds` (off by default) |
| `MAX_MODELS_PER_PROJECT` | Hard cap on filtered models per request |
| `ROLE_AGNOSTIC_SCHEMA_CACHE` | One pre-connection schema per project (role omitted from cache key); superset GraphQL shape; resolver enforces read/function permissions |

## Golden snapshots (schema shape)

`controller/public_schema_introspection_golden_test.go` compares normalized introspection JSON to `controller/testdata/introspection_*.golden.json`.

**Intentional GraphQL shape change:** run from `open-core` with a running schema (same codegen assumptions as production):

```bash
UPDATE_PUBLIC_SCHEMA_GOLDEN=1 go test ./controller/ -run TestPublicSchemaIntrospectionGolden -count=1
```

Review the diff, commit updated `*.golden.json` files with the behavioral change.

**Large fixture:** `TestPublicSchemaIntrospectionGolden_ten_models_long` is skipped under `go test -short`; run without `-short` or in a dedicated job.

## Metrics (when `SCHEMA_BUILD_METRICS=true`)

- `schema_build_total{result}` — `hit` / `miss` when compiled cache is enabled (pre-connection cache hit vs miss), or `error` on builder failure.
- `schema_build_duration_seconds` — histogram with `result` = `success` or `error`.

Example PromQL:

```promql
sum(rate(schema_build_total[5m])) by (result)
histogram_quantile(0.95, sum(rate(schema_build_duration_seconds_bucket[5m])) by (le, result))
```

## Role-agnostic schema cache

When `ROLE_AGNOSTIC_SCHEMA_CACHE=true`, the builder uses an admin-equivalent permission map so the **GraphQL shape is a union** of what any role could see. The **pre-connection LRU key omits role** so the same entry serves all roles. **Resolvers** enforce the real role:

- Read queries: `enforceRoleAgnosticModelRead` in `resolver/public_schema_query.go` (blocks `Read: none`).
- Functions: `ApitoFunctionResolverFn` requires `LogicExecutions` or admin (unless role-agnostic is off).
- Mutations: existing `MutationResolverFn` permission checks remain authoritative.

**Introspection:** clients see the full union; if per-role schema hiding is required, keep this flag off or add a separate introspection policy.

# open-core changelog

Notable changes to **open-core** only. Release tags use `v1.x` (for example `v1.5.4`, `v1.6.0`).

## [1.8.18] - 2026-08-29

### Removed

- Dropped `PaddleWebhookSecret` from `Config`. Hosted-provider secrets live on pro `ProConfig`, not open-core.

## [1.8.17] - 2026-08-29

### Added (reverted in 1.8.18)

- Briefly added a payment webhook secret field; that belongs in pro, not this module.

## [1.8.16] - 2026-08-22

### Added

- **`SystemUser.IsSuperAdmin`** — persisted `is_super_admin` column (distinct from bootstrap `IsAdmin` and project `Role.IsAdmin`).
- **`TokenClaims.IsSuperAdmin`** — copied from JWT `is_super_admin` (`"true"` / `true` / `"1"`) and stamped on `apt_` from the issuer row. Never set on `ak_`.
- JWT mint sets `is_super_admin` only from the persisted flag (not `IsAdmin` alone). `is_admin` remains for legacy profile claims.
- `SetTokenClaimsToRouter` stores the claims pointer so operator gates can require the token claim **and** the DB flag.

## [1.8.15] - 2026-08-12

### Fixed

- **Google login** — refuse create when ID token has no email (stops empty-email orphan users).
- **Google login** — when an existing `google_sub` row has empty email, backfill from the token before minting.

## [1.8.14] - 2026-08-12

### Changed

- **`getProjectPlans`** — requires `CapPlansRead` only (no project-admin); login/SDK keys can load prices + `provider_products`.
- **Login capability preset** — includes `CapPlansRead`.
- **`NormalizePlanSlug`** — maps legacy `pro` / `business` → `paid`.

## [1.8.13] - 2026-08-11

### Added

- **Idle tenant project settings** — `idle_tenant_retention_days` (default/min **90**) and `auto_soft_delete_idle_tenants` on `ProjectSettings` + `UpdateSettings` GraphQL input. Helper `EffectiveIdleTenantRetentionDays`.
- **Plan monetization / seed** — plan system flags and related GraphQL plan mutations / seed test updates.
- **Physical schema health** — interfaces + resolver/services for project physical schema health checks.

## [1.8.12] - 2026-08-11

### Notes

- Tag pinned by engine Studio prep (see engine `changelogs/2026-08-11.md`). Changelog catch-up for prior pin.

## [1.8.11] - 2026-08-07

### Fixed
- `upsertModelData` with an explicit `_id` now **inserts** when the document is absent instead of failing with `document <id> not found`. Previously the supplied-id branch was update-only, so `apito sync --type content` into a fresh destination died on the first row. Drivers disagree on how "missing" is reported (sqlite/mongo/bbolt error, postgres/mysql/mariadb return an empty document with a nil error); both are now normalized as absent, which also removes the misleading `document does not belongs to <model>` on SQL drivers.

### Added
- `ae.ErrDocumentNotFound` sentinel plus `resolver.IsDocumentNotFoundErr` so callers match missing documents with `errors.Is` instead of driver message strings.

## [1.8.10] - 2026-08-05

### Added
- Multi-provider project app-user OAuth settings: Facebook, GitHub, X, LinkedIn (flat fields parallel to Google).
- GraphQL `has_*_client_secret` for each provider; `oauthState(provider, project_id)` (+ `googleOAuthState` alias).
- `loginUser` `auth_method` values: `facebook`, `github`, `x`, `linkedin` (code+state exchange).
- Project auth users `oauth_sub` column + list-by-provider-sub for non-Google linking.

## [1.8.9] - 2026-08-05

### Fixed
- `upsertConnectionToModel` always normalizes connection `Type` to `forward`/`backward` on update (flipped metadata no longer sticky).

## [1.8.7] — 2026-08-04

### Fixed

- **Public create/update connect DDL** — `MutationResolverFn` sets
  `ProjectSchemaModels` (same as public queries / system upsert) so SQLite
  `EnsureRelationArtifactsFromSchema` runs before connect. Stops
  `no such column: <model>_id` on tenants whose tables predate relation FKs
  (e.g. `createExam` + `mark_config_ids` → `exam_id`).
- **Orphan docs on failed connect** — public `createAndConnectDocument`
  deletes the inserted document if connect param build or `ConnectBuilder`
  fails (parity with system upsert).
- **Auth settings cache lag** — `updateProjectAuthenticationSettings`
  refreshes `ProjectCache`; `loginUser` prefers DB-backed project for auth
  identifier method so settings apply immediately.

## [1.8.6] — 2026-07-30

### Breaking — Nested public GraphQL relations are snake_case

- **has_one nested fields** use `RelationFilterGraphQLKey` → canonical stored model id (`food_category`), not `SingularResourceName` (`foodCategory`).
- **has_many nested fields** use `RelationNestedListGraphQLKey` → `{model}_list` (`food_category_list`), not `MultipleResourceName` (`foodCategoryList`).
- **Root** query/mutation names stay lowerCamel (`foodList`, `foodCategoryList`).
- Pair with admin SDKs ≥ js **3.11.7** / matching go+flutter naming helpers. Deploy engine built against this tag before shipping SDK consumers to production.

## [1.8.5] — 2026-07-27

### Fixed

- **Canonical long model ids** — `CanonicalizeModelName` skips run-on rejection when the input already matches canonical snake_case (`indication`, `practitioner`). CLI/schema sync no longer fails `model name needs a word boundary…` on valid stored ids.
- **Draft field/connection ops** — `UpsertFieldToModel` / connection resolvers fall back to `LegacyStoredNameToCanonical` when canonicalize fails, so draft-only long singles still accept field ops.
- **Empty repeated on full replace** — `HandlePayloadFormatting` writes `[]` for empty repeated arrays when `deltaUpdate=false` (clear nested items); empty input remains a no-op under delta update.

## [1.8.3] — 2026-07-22

### Fixed

- **Recursive `SubFieldInfo` in system GraphQL** — `projectModelsInfo` / FieldInfo no longer truncates at a leaf `NestedSubFieldInfo`. Nested `repeated`/`object` trees (e.g. `exam.routine.details.date_and_time`) are returned for CLI/MCP schema sync.
- **Empty nested where filters** — empty nested `repeated`/`object` groups no longer abort public schema generation with `…COMMON_FILTER_CONDITION fields must be an object`; a placeholder input field is used until children exist.

## [1.8.2] — 2026-07-22

### Fixed

- **`deleteModelData` nil panic** — set `ResolveParams` before `GetSingleProjectDocument` in both delete resolvers (delete from CMS no longer returns `invalid memory address or nil pointer dereference`). Pair with open_driver nil-safe `ResolveParams` when reading `local`.

## [1.8.1] — 2026-07-22

### Fixed

- **Parent field lookup** — `searchAndOperateOnFields` prefers the shallowest match for `parent_field` before nested groups with the same identifier (e.g. `class.sections` vs `class.divisions.sections`). Stops schema sync from adding nested fields onto the wrong parent when identifiers collide.

## [1.8.0] — 2026-07-21

### Breaking — Unified `apt_` access tokens + fail-closed project scope

- **Access tokens** — `apt_` principals authenticate via `Authorization: Bearer` + `X-Use-Cookies: false`. Legacy `cli-`/`sdk-`/`mcp-` sync-key prefixes are rejected (`TOKEN_FORMAT_RETIRED`). Token create/list/revoke/rotate stays on Console session only.
- **Project header** — canonical scope header is `X-Apito-Project-Id` only. Access-token principals never inherit `ProjectID` from a sole grant; missing/invalid project scope fails closed.
- **Capabilities** — data GraphQL, secured REST/files, and schema/role/plugin/function/tenant resolvers enforce access-token capability gates; pro tenant resolvers included.
- **Session override** — cookie Console sessions may still use `applySessionProjectOverride` for multi-project UIs; `apt_` principals do not.

### Added

- Access-token models/service/policy, capability registry, and matrix tests under `services/` + `authz/`.

## [1.7.14] — 2026-07-20

### Apito Functions

- **`active_revision_hash`** — CloudFunction GraphQL field; `projectFunctionsInfo` enriches from the active revision’s `artifact_hash` (SHA-256 of deployed source) for CLI sync live-parity diffs. Not persisted on the function row.

## [1.7.13] — 2026-07-18

### Apito Functions (tenant-safe SaaS + lifecycle)

- **Tenant scope** — `FunctionTenantScopeHook` / `FunctionCallableAuthHook`; stable codes `TENANT_REQUIRED`, `TENANT_NOT_FOUND`, `TENANT_NOT_ACTIVE`, `TENANT_SCOPE_FORBIDDEN`, `TENANT_DB_NOT_READY`.
- **Draft test** — `testFunctionDraft` validates tenant before Deno/SQL; typed Pro routing keys (no plain-string `tenant_id` context miss).
- **Live SaaS REST** — `/function` requires app-user Bearer JWT + `X-Fn-Hash`; tenant from claims only.
- **Authz** — `requireFunctionManage` on list/upsert/delete/test/deploy/history/rollback.
- **Lifecycle GraphQL** — deploy/rollback/revisions/deployments; Deno gateway read ops + embedded SDK bridge.

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

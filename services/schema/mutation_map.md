# Schema mutation participant map

Synchronous GraphQL schema mutations and their participants (before orchestration refactor).

## createModel — `AddModelToProjectResolverFn`

| Step | Participant | Call |
|------|-------------|------|
| 1 | Validate | `CanonicalizeModelName`, `CheckTableOrCollectionExists` |
| 2 | Base project DDL | `driver.AddModel` (updates in-memory `project.Schema`) |
| 3 | Tenant DDL | `SchemaIterateHook` → **not wired in pro boot** (was `_ =` when set) |
| 4 | System schema | `SystemDriver.UpdateProject(..., false)` |
| 5 | Cache | `refreshProjectAndReCache` |

**Gap:** System write after DDL; no tenant propagation; no compensation if system write fails.

## addField / updateField — `UpsertFieldToModelResolverFn`

| Step | Participant | Call |
|------|-------------|------|
| 1 | Validate | In-memory field build + duplicate check |
| 2 | Base project DDL | `driver.AddFieldToModel` |
| 3 | Tenant DDL | `SchemaIterateHook` → **errors ignored** (`_ =`) |
| 4 | System schema | `SystemDriver.UpdateProject(..., true)` |
| 5 | Cache | `ExpireGraphQLFieldCache`, `refreshProjectAndReCache` |

## modelFieldOperation — `ModelFieldOperationResolverFn`

| Operation | Base DDL | Tenant DDL | System persist | Cache |
|-----------|----------|------------|----------------|-------|
| rename | `RenameField` | hook ignored | `UpsertModelType` | `ExpireGraphQLProjectCache` |
| duplicate | in-memory only | — | `UpsertModelType` | project cache |
| change_type | in-memory only | — | `UpsertModelType` | project cache |
| delete | `DropField` (**errors logged, not returned**) | hook ignored | `UpsertModelType` | project cache |

**Gap:** Delete mutates in-memory schema before DDL; DDL failure still persisted metadata.

## createConnectionType — `CreateConnectionTypeResolverFn`

| Step | Participant | Call |
|------|-------------|------|
| 1 | Validate | Models, fields, connection metadata in memory |
| 2 | Base project DDL | `driver.AddRelationFields` |
| 3 | Tenant DDL | **not called** |
| 4 | System schema | `TouchProjectUpdatedAt`, `UpsertModelType` ×2 |
| 5 | Cache | `refreshProjectAndReCache` |

## Target ordering (orchestrator)

1. Validate / plan
2. Base project DDL
3. Tenant DDL (`SchemaIterateHook`)
4. System schema persistence
5. Cache refresh

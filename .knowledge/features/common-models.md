---
type: feature
title: Common Models (open-core)
description: Promote-only toggle, paginated verified copy, shadow repair, hide-deleted queries
resource: resolver/common_model_reconcile.go
tags: [open-core, saas, sqlite]
timestamp: 2026-09-07T00:00:00Z
---

# Common Models (open-core)

## Purpose

Open-core runs promotion, shadow repair, and query rewriting without importing `open_driver` (no cycle). Drivers expose `ReduceModelTableToShadow`, `ListDistinctNonEmptyColumn`, `InsertShadowIDs`.

## Flows

- **`validateCommonModelToggle`:** reject `wantCommon=false`; strategy must be `promote` or empty; empty + existing rows requires `promote`.
- **`runCommonModelPromotion`:** page size 500 (`SetCommonModelPromotionPageSizeForTest`). Skip IDs already in base; conflict if same ID has different `Data`; verify all IDs in base; **then** reduce. Resume = skip existing, reduce again.
- **`repairCommonShadows`:** tenant `has_one` FKs + `has_many` pivots → insert live base IDs missing from shadow; GC base-missing/deleted orphans; count real `shadows_fixed`.
- **`prepareCommonModelQuery`:** rewrite relation filters; error on common-relation sort. Public + system list/count.
- **`NewParam(nil)` panics** — reconcile allocates `{ProjectID: project.ID}` when parent is nil.

## Main files

- `resolver/common_model_promote.go`, `common_model_reconcile.go`, `common_shadow_refs.go`, `common_relation_filter.go`
- `models/saas_scope.go` — `ModelIsCommon`, `WithBaseProjectDB`
- `executor/common_hydrate.go` — nested hydrate from base
- `resolver/system_mutations.go` — `updateModel` guard + GraphQL arg text

## Dependencies

Engine calls `GraphQLServer.ReconcileCommonModels`. SQL via `ProjectDBInterface` peel, not a direct `open_driver` import.

## Invariants

- Do not mark Ext common until promotion succeeds.
- Do not reduce if verification fails.
- Shadow helpers duplicated in open-core (do not import `sqlquery`).

## Common bugs

- Importing `open_driver` from open-core (layering).
- Treating 0 rows as “skip reduce” (new tenants still need id-only tables).
- Soft-delete only on nested hydrate, not root list/count.

## Tests

`go test ./resolver/ -run 'Promote|Shadow|Relation'`

## Related

- `apito/.knowledge/features/common-models.md`
- `open_driver` SQLite reduce + `sqlcommon/shadow.go`

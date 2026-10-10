# open-core — Current

## Relation transition API (2026-10-11)

- `relationmigrate` classifies shape, safety, reversibility, and `plan_hash`.
- GraphQL `previewSchemaRelationTransition` is read-only. `stageRelationTransition` requires that hash and the engine acknowledgement.
- `upsertConnectionToModel` rejects a storage-changing edit of an existing edge.
- Non-versioned projects run the same migrator. Not tagged. `:5050` unchanged until restart. Ask before commit / `./command.sh save`.

## Unique keys on the model (2026-10-05)

- `ModelType.UniqueKeys` and GraphQL `unique_keys`. Mutations `upsertUniqueKey` and `deleteUniqueKey` stage when versioning is on.
- `ValidateUniqueKey` allows a root field or a `has_one` FK (`variant_id`). At least two columns.
- Empty list means no composite index. Not tagged. Ask before commit / `./command.sh save`.

## Upsert connect error kept (2026-09-23)

- `UpsertModelDataFnFn` no longer assigns `DeleteDocumentFromProject` over the connect error. A bad relation returns the relation error after the orphan is removed.
- Source only. Local `__debug_b` on `:5050` still runs the old binary until restart.
- Not tagged. Ask before commit / `./command.sh save`.

## Rosna food-order create trace (2026-09-23)

- `createAndConnectDocument` logs payload keys, connect counts, format/insert/connect/total ms, create hook names. `food_order` only.
- Debug helper writes NDJSON to `.cursor/debug-a9fbe8.log`. Remove after the latency pass.
- Not tagged. Engine on `:5050` must restart to load it.

## Platform Google login + staff bootstrap gate (2026-09-13)

- `BuildSystemParam` role-template merge preserves request
  `IsProjectUser` and `ReadOnlyProject`.
- Persist stripping keeps the hidden `users` model when inverse connections
  are active.
- Secured `ak_` login/bootstrap keys 403 `BOOTSTRAP_KEY_ON_SECURED`;
  website tenant tokens remain allowed.
- Removed temporary token middleware and ACL authorization debug
  instrumentation/helper.
- Utility tests pass and `services` compiles. Full services tests fail only at
  pre-existing timing-sensitive `TestCustomTokenWithTTL`.

## Working on

- Common models: promote-only, paginated verify-then-reduce, shadow
  repair, hide-deleted queries.

## Live

- Cutting **v1.8.24** with engine **v2.4.55**. Previous live: **v1.8.23** /
  engine **v2.4.53**.

## Do not

- Import `open_driver` from this module.
- Advertise demotion.
- Drop `TrialEndsAt` from `EmailSendRequest`.

## Last Updated

2026-09-13

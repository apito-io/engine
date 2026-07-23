# open-core — Handoff

## Branch

- `main` clean — tagged **v1.8.3** (pushed GitHub `apito-io/engine` module)

## Done (2026-07-22 — v1.8.3)

- `GetFieldInfoObject`: recursive `SubFieldInfo` via FieldsThunk (no leaf
  `NestedSubFieldInfo`)
- `BuildWhereConditionArgument`: empty nested groups → `_empty` Boolean
- Tests: recursive type + empty nested repeated/object filters

## Broken / watch

- Public schema still builds filters for empty groups (placeholder only);
  remediating empty children via sync is the real fix

## Next

- Path-aware `parent_field` if products keep colliding identifiers

## Do not touch

- Import `open_driver` / `pro_driver` / `plugin_system` / `nats_system` from open-core

## Last Updated

2026-07-23

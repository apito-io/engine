# Handoff — open-core 2026-09-03

## Uncommitted

- Plugin activation helpers + tests.
- `upsertPlugin` honors enable, does not invert activate_status, appends
  new plugins, expires project cache. Does not rewrite DefaultStoragePlugin.
- `project.rest` REST 403 unless activated + project id.
- YAML capabilities; legacy `type: project` maps (strict in dev).
- GraphQL `capabilities` / official bundle fields. `type` always system.

## Do not

- Reuse protobuf type enum 0/1.
- Change HashiCorp five-method interface.

## Last Updated

2026-09-03

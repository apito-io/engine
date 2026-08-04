# open-core — Handoff

## Branch

- `main` — tag **v1.8.7** (2026-08-04)

## Done (2026-08-04 — v1.8.7)

- Public `MutationResolverFn` sets `ProjectSchemaModels`.
- Orphan delete after failed connect on public create.
- Auth settings: ProjectCache refresh + loginUser DB prefer.

## Broken / watch

- Older tenant DBs still need one ensure/connect to ALTER missing FK columns;
  v1.8.7 makes public mutations run that path.

## Next

- Engine require **v1.8.7** + Studio rebuild from engine tip.

## Do not touch

- Import `open_driver` / `pro_driver` / `plugin_system` / `nats_system` from open-core

## Last Updated

2026-08-04

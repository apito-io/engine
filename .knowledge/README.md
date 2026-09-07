# open-core — Knowledge

Part of the **apito** ecosystem. See `/.knowledge/projects/apito.md` for how this repo fits and its blast radius.

## Read order
1. This file. 2. `ARCHITECTURE.md`. 3. `DECISIONS.md`. 4. `features/README.md`. 5. `../.memory/CURRENT.md` and `../.memory/HANDOFF.md`.

## Purpose
Open-core engine library (`github.com/apito-io/engine`): GraphQL system/public schemas, schema orchestrator, document CRUD, and SaaS-agnostic helpers. Pro engine embeds this module.

## Responsibilities
- System + public GraphQL resolvers, schema change saga, auth users in project DB
- Common-model promotion/repair/query rewrite without importing `open_driver` (see `features/common-models.md`)

## Consumers / blast radius
Pro engine, SDKs via schema, console codegen. Pin tags — Studio builds comment local `replace`.

## Reasoning archive
- Historical Cursor plans distilled into this knowledge base live in `archive/plans/`.

Last Updated: 2026-07-06

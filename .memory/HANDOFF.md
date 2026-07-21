# open-core — Handoff

## Branch
- Tracks engine `udbhabon` via go.mod replace in monorepo

## Done
- **apt_ tokens (2026-07-20→21):** Full AccessTokenService + authz + middleware +
  REST; `applySessionProjectOverride` for Console multi-project GraphQL.
  Uncommitted — ask before `./command.sh save`.
- **Apito Functions foundation (2026-07-16):** `functions/*` runtime stack;
  active_revision_hash (v1.7.14). Write ops stubbed.

## Broken / watch
- Capability bindings catalog ≠ full resolver enforcement yet
- Data gateway write ops still stubs
- Existing DB needs `EnsureSystemUserAccessTokensColumn` (open_driver migration)

## Next
- Capability gate rollout per ACCESS_TOKENS checklist
- Do not remove HashiCorp `hc-*` path until Rosna functions validated

## Do not touch
- Import `open_driver` / `pro_driver` / `plugin_system` / `nats_system` from open-core
- Generated GraphQL in console — edit `.ts` documents + codegen

## Last Updated
2026-07-21

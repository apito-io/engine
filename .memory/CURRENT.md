# open-core — Current

## Working on
- **apt_ fail-closed project scope (2026-07-21):** Canonical
  `X-Apito-Project-Id` only (no aliases). Shared `applyAccessTokenScope` on both
  auth branches; access_token claims never pick `ProjectIDs[0]`; listProjects
  grant-filtered; token mint/list/revoke/rotate console-session only; capability
  gates on secured GraphQL data + REST/files + schema/role/plugin/function/tenant
  ops. Uncommitted — ask before save. Deep design: `engine/.knowledge/ACCESS_TOKENS.md`.
- **Apito Functions (2026-07-16):** Runtime-neutral `functions/` package landed —
  contracts, Deno + wazero providers, local/NATS transport, artifact store,
  lifecycle, data gateway (write ops still stubbed).

## Next
- Tag open-core release when apt_ + scope ship
- Wire remaining FunctionDataGateway write/batch ops

## Last Updated
2026-07-21

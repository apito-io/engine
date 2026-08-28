# open-core — Current

## Working on

- **Platform super-admin (2026-08-22, uncommitted):** changelog **1.8.16**.
  `SystemUser.IsSuperAdmin`, `TokenClaims.IsSuperAdmin`, JWT mint from
  the flag only, `apt_` stamped at validate-time, `ak_` never.
  `AllowUnscopedSystemUserList` + search helpers.

## Released

- **v1.8.15:** Google login refuse empty email + backfill.
- **v1.8.11:** `upsertModelData` supplied-`_id` insert.

## Next

1. Tag **v1.8.16** first in hasher cascade (awaiting execute)
2. Do not ship synthetic-admin rewrite (still deferred)

## Last Updated

2026-08-28

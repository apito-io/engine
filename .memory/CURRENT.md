# open-core — Current

## Working on

- **Scoped bootstrap (2026-08-09, uncommitted):** `RegisterUser` hook on
  `ProjectUserGraphQLHooks`; caps `auth.login`/`auth.register`; preset
  `sdk_bootstrap`; login/register/createUser gates; `token.go` fix so
  `ak_` + `X-Use-Cookies: false` uses apiKeyManager (not VerifyIDToken).
  Engine must restart after rebuild.

## Released

- **v1.8.11 (tagged 2026-08-07):** `upsertModelData` supplied-`_id` insert.
- **v1.8.10:** Multi-provider OAuth + `oauth_sub`.
- **v1.8.9:** relation Type normalize.

## Next

1. Tag after smoke + user confirm (likely v1.8.12)
2. Do not ship synthetic-admin rewrite yet (deferred)

## Last Updated

2026-08-09

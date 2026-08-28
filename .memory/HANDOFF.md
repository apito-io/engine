# Handoff — open-core 2026-08-22

## Done (uncommitted)

- `SystemUser.IsSuperAdmin` + `TokenClaims.IsSuperAdmin`
- JWT mint `is_super_admin` from persisted flag; parse `"true"` / `true` / `"1"`
- `AccessTokenService.ValidateRaw` stamps issuer flag; `ak_` round-trip false
- `SetTokenClaimsToRouter` stores claims pointer
- `AllowUnscopedSystemUserList` / `HasSystemUserSearchFilter`

## Next

- Tag **v1.8.16** first in hasher cascade (awaiting execute; engine pin after)

## Do not

- Mint `is_super_admin` from `IsAdmin` alone
- Auto-tag without ask


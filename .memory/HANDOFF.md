# Handoff — open-core 2026-09-13

## Platform Google login + staff bootstrap gate

- Role resolution copies the persisted template while preserving request
  `IsProjectUser` and `ReadOnlyProject`; authenticated passenger/staff ACL
  therefore sees the caller as a project user.
- `StripProjectAuthUserModelFromPersistedSchema` removes only unconnected
  runtime users descriptors and retains active inverse relation metadata.
- Secured `ak_` login/bootstrap keys 403 `BOOTSTRAP_KEY_ON_SECURED`;
  website tenant tokens remain allowed. `/system/graphql` still accepts
  login keys.
- Temporary token/ACL debug instrumentation and helper were removed.
- Utility tests pass; `services` compiles. Full services tests retain only the
  pre-existing timing-sensitive `TestCustomTokenWithTTL` failure.

## Common models (v1.8.23)

- `validateCommonModelToggle` promote-only.
- `runCommonModelPromotion` page 500, conflict, verify, then reduce.
- `repairCommonShadows` real `shadows_fixed`.
- `prepareCommonModelQuery` list+count; reject common-relation sort.
- Docs: `features/common-models.md`.

## Do not

- Import `open_driver`.
- Use `NewParam(nil)` (panics).
- Reject `website` tenant tokens on `/secured/graphql`.

## Last Updated

2026-09-13

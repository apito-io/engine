# open-core — Current

## Platform Google login + staff bootstrap gate (2026-09-13)

- `BuildSystemParam` role-template merge preserves request
  `IsProjectUser` and `ReadOnlyProject`.
- Persist stripping keeps the hidden `users` model when inverse connections
  are active.
- Secured `ak_` login/bootstrap keys 403 `BOOTSTRAP_KEY_ON_SECURED`;
  website tenant tokens remain allowed.
- Removed temporary token middleware and ACL authorization debug
  instrumentation/helper.
- Utility tests pass and `services` compiles. Full services tests fail only at
  pre-existing timing-sensitive `TestCustomTokenWithTTL`.

## Working on

- Common models: promote-only, paginated verify-then-reduce, shadow
  repair, hide-deleted queries.

## Live

- Cutting **v1.8.24** with engine **v2.4.55**. Previous live: **v1.8.23** /
  engine **v2.4.53**.

## Do not

- Import `open_driver` from this module.
- Advertise demotion.
- Drop `TrialEndsAt` from `EmailSendRequest`.

## Last Updated

2026-09-13

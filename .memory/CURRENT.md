# open-core — Current

## Working on

- Email: `ComposeProjectTransfer` + `QuotaNote`. Noop sender logs
  `AcceptURL` when present. Do not drop `TrialEndsAt`. Uncommitted
  until engine cut.

## Live

- **v1.8.21** with engine **v2.4.51**. Live with `engine:v1.14`.

## Also uncommitted

- System plugin refactor: capabilities, activation upsert, REST gate.

## Do not

- Mint `is_super_admin` from `IsAdmin` alone.
- Restore `PLUGIN_TYPE_PROJECT` as a user-facing install class.
- Drop `TrialEndsAt` from `EmailSendRequest` again.

## Last Updated

2026-09-07

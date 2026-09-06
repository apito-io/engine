# Handoff — open-core 2026-09-07

## Uncommitted — project transfer email

- `ComposeProjectTransfer` in `services/email.go`.
- `AcceptProjectTransferURL` in `models/invite.go`.
- `EmailSendRequest` has `ActivateBy`, `TrialEndsAt`, **`QuotaNote`**.
- Noop sender logs `accept <url>` so local runs without Gmail still
  show the link in engine stdout.
- `FormatReadableDate` in `utility/date.go`.

## Also uncommitted

- Plugin activation helpers + REST gate + YAML capabilities.

## Do not

- Reuse protobuf type enum 0/1.
- Change HashiCorp five-method interface.
- Drop `TrialEndsAt` on `EmailSendRequest`.

## Last Updated

2026-09-07

# Handoff — open-core 2026-10-11

## Fair Source

- **Done:** License packet, public plan type without payment fields, local schema draft routes, read-only project SQL, release workflow.
- **Next:** Counsel signs `LEGAL_REVIEW.md` before any public FSL tag. Do not regenerate GraphQL by hand.
- **Do not:** Sign the review. Describe this tree as OSI open source. `./command.sh save` until asked.

## Relation transition

- **Done:** Preview contract, GraphQL query/mutation, upsert guard. Conflict policy is `abort`.
- **Next:** Engine restart before Console or MCP can call the new fields.
- **Do not:** Offer keep-first or keep-latest. Tag from this alone. `./command.sh save` until asked.

# Handoff — open-core 2026-10-05

## Unique keys

- **Done:** Schema type, staged mutations, validation. Versioning hook skips the live write.
- **Next:** Console can call the mutations without codegen. Engine publish applies them.
- **Do not:** tag from this alone. Do not `./command.sh save` until confirmed.

## Upsert connect error

- **Done:** New-document connect failure in `system_mutations.go` uses `_ = Delete` then `return nil, err`, matching the public mutation path.
- **Next:** Restart the local engine debugger before expecting the new error text.
- **Do not:** tag a release from this alone. Do not `./command.sh save` until confirmed.

## Rosna food-order create trace

- Logs only when model is `food_order`. No behavior change besides the log write.
- Next: restart local engine, one POS draft, then read phase timings.
- Do not remove `food_order_debug_log.go` until the trace is read. Do not tag.

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

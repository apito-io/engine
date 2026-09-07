# Handoff — open-core 2026-09-07

## Common models (v1.8.23)

- `validateCommonModelToggle` promote-only.
- `runCommonModelPromotion` page 500, conflict, verify, then reduce.
- `repairCommonShadows` real `shadows_fixed`.
- `prepareCommonModelQuery` list+count; reject common-relation sort.
- Docs: `features/common-models.md`.

## Do not

- Import `open_driver`.
- Use `NewParam(nil)` (panics).

## Last Updated

2026-09-07

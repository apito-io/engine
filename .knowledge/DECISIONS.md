# open-core — Decisions

Format: Decision / Reason / Alternatives / Status. Newest on top.

---

### Future engine releases in this tree are Fair Source
- **Reason:** Public self-hosting is a complete builder. Cloud sells operations. See root `.knowledge/DECISIONS.md`.
- **Status:** Accepted (2026-10-11). `LICENSE` is FSL-1.1-ALv2. Tags through `v1.8.25` remain Apache 2.0. Counsel sign-off is `LEGAL_REVIEW.md`.

### Common models are promote-only
- **Reason:** Demotion would copy base rows back into every tenant DB and is not implemented. Advertising it caused operators to expect a migration that does not exist.
- **Alternatives:** Bidirectional migrate; hide the toggle after promote.
- **Status:** Accepted (2026-09-07). Strategy must be `promote`. SQLite/libSQL is the certified path.

---
_TODO: distilled from completed plans in `archive/plans/`._

Last Updated: 2026-09-07

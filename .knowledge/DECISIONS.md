# open-core — Decisions

Format: Decision / Reason / Alternatives / Status. Newest on top.

---

### Common models are promote-only
- **Reason:** Demotion would copy base rows back into every tenant DB and is not implemented. Advertising it caused operators to expect a migration that does not exist.
- **Alternatives:** Bidirectional migrate; hide the toggle after promote.
- **Status:** Accepted (2026-09-07). Strategy must be `promote`. SQLite/libSQL is the certified path.

---
_TODO: distilled from completed plans in `archive/plans/`._

Last Updated: 2026-09-07

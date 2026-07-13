# open-core — AI Changelog

Not git history — the *reasoning* behind changes. Newest on top.
Format per entry: date, **Changed**, **Why**, **Affected**.

---
## 2026-07-13 — v1.7.11
- **Changed:** `ModelIsSaaSTenantControlPlaneModel`, hook takes `*[]*PublicSchemaModelFilter`, skip tenant model in `collectFilteredModelsForPublicSchema`, mutation meta nil guards.
- **Why:** Engine pro SaaS tenant delete/catalog journey; studio clone builds against tagged open-core, not monorepo replace.
- **Affected:** `models/project_tenant_model.go`, `models/config.go`, `controller/public_schema_builder*.go`, `executor/solver.go`, `resolver/public_schema_mutation.go`.

## 2026-07-06
- **Changed:** Bootstrapped knowledge system for this repo.
- **Why:** Cross-LLM durable knowledge + working memory.
- **Affected:** this repo only.

Last Updated: 2026-07-13

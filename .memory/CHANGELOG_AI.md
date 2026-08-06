## 2026-08-06 — v1.8.10 multi-provider OAuth auth settings + login

- **Changed:** Flat FB/GitHub/X/LinkedIn settings + GraphQL; `oauthState`;
  `loginUser` code exchange; `oauth_sub` on project auth user model;
  helpers/tests.
- **Why:** Match Google capability for other providers; Console tabs.
- Tag **v1.8.10** pending confirm.

## 2026-08-05 — v1.8.9 upsertConnection normalizes forward/backward

- **Fixed:** CreateConnectionTypeResolverFn always sets from=`forward`, to=`backward` on update.
- **Why:** Flipped Type metadata made CLI/pro staging treat edges as present but never rewrite direction.
- Tag **v1.8.9**.

---
# open-core — AI Changelog

## 2026-08-05 — v1.8.8 FieldIsSortable

- **Changed:** `utility.FieldIsSortable`; restrict public sort payload fields.
- **Affected:** field_sortable.go, public_schema_builder_build.go. Tag **v1.8.8**.

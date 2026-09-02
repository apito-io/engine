## 2026-09-02 — permission rewrite + partner caps (v1.8.19)

- **Changed:** `RewriteProjectPermissionKeys` / `MoveAPIPermissionKey`;
  `renameModel` + naming v2 migration use them. Capability registry
  binds `searchPartners` / `createPartner` / `updatePartner`.
- **Why:** Schema publish stages rename without open-core `renameModel`,
  so staff roles kept old model ids. Partners GraphQL needs authz.
- **Affected:** `utility/permission_builder.go`, `resolver/system_mutations.go`,
  `authz/capability_registry.go`. Tagged **v1.8.19**.

---

## 2026-08-22 — Platform super-admin claim (1.8.16 unreleased)

- **Changed:** `SystemUser.IsSuperAdmin` + `TokenClaims.IsSuperAdmin`.
  JWT/`apt_` mint from the persisted flag only; never on `ak_`.
  `AllowUnscopedSystemUserList` for platform account search.
- **Why:** `IsAdmin` is bootstrap/profile, not platform operator.
- **Affected:** `models/user_project.go`, `models/api_token.go`,
  `services/jwt.go`, `services/access_token_service.go`,
  `utility/claims_set.go`. Ask before tag **v1.8.16**.

---

## 2026-08-12 — Google login: refuse empty email + backfill

- **Changed:** `ResolveUserForGoogleLogin` errors on missing email (no
  `createFn` path); backfills email when `google_sub` hit has empty email.
- **Why:** Shared-DB SaaS (Kisti) was creating blank Google users → 403.
- **Affected:** `resolver/user_google_login.go` + tests. Tag **v1.8.15**.

---

## 2026-08-10 — Roles/permissions + ak\_ lifecycle harden

- **Changed:** Project-admin gates on role/token CRUD; `ProjectToken`
  metadata (`token_id`/`prefix`/`fingerprint`) + blacklist on validate;
  v2 role encoding; fail-closed scopes (`none|all|auth|own`, no
  `custom_logic`); `EffectivePermission`/`AuthorizeModel*`; SQL own on
  counts; dataloader + known_as target read; system GraphQL capability
  gate; `project_tokens` SQL column migration.
- **Why:** Close privilege escalation, revoke bypass, fail-open scopes.
- **Affected:** `resolver/*`, `utility/*`, `services/*`, `authz/*`,
  `controller/graph_controller.go`; open_driver system SQL migrations.
  **Ops:** restart engine (migrate), then Console `pnpm codegen`.
  Ask before commit/tag.

---

## 2026-08-09 — Scoped bootstrap: RegisterUser + auth caps + ak\_ token path

- **Changed:** `RegisterUser` on GraphQL hooks; caps `auth.login` /
  `auth.register`; preset `sdk_bootstrap`; gates on login/register/
  createUser; `token.go` treats `ak_` + `X-Use-Cookies:false` as API key
  (not ID token). Security tests added.
- **Why:** Non-admin Flutter/SDK keys can signup/login; createUser stays
  privileged; cookie=false clients were 403’ing on login.
- **Affected:** `authz/*`, `resolver/user_*.go`, `services/token.go`,
  tests. Ask before tag (likely **v1.8.12**).

---

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

## 2026-08-07 — v1.8.11 upsertModelData inserts with supplied \_id

- **Fixed:** `UpsertModelDataFnFn` supplied-`_id` branch was update-only; missing
  doc returned `document <id> not found` (sqlite) or `document does not belongs
to <model>` (postgres/mysql/mariadb empty-doc + nil error). Now falls through
  to insert with that id, preserving cross-environment id parity.
- **Why:** `apito sync --type content` to an empty destination aborted on the
  first row; relation sync depends on source ids existing on the destination.
- **Added:** `ae.ErrDocumentNotFound` + `resolver.IsDocumentNotFoundErr`;
  `existingDocumentFromLookup` normalizes both driver conventions.
- **Affected:** `err/system.go`, `resolver/document_lookup.go`,
  `resolver/system_mutations.go`. Tag **v1.8.11**.

---

# open-core — AI Changelog

## 2026-08-05 — v1.8.8 FieldIsSortable

- **Changed:** `utility.FieldIsSortable`; restrict public sort payload fields.
- **Affected:** field_sortable.go, public_schema_builder_build.go. Tag **v1.8.8**.

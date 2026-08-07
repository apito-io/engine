# open-core — Current

- **v1.8.11 (tagged 2026-08-07):** `upsertModelData` with an explicit `_id`
  now inserts when the document is absent (was update-only, so content sync
  into a fresh destination failed with `document <id> not found`). New
  `ae.ErrDocumentNotFound` + `resolver.IsDocumentNotFoundErr`;
  `existingDocumentFromLookup` normalizes sqlite/mongo/bbolt errors vs
  postgres/mysql/mariadb empty-doc-with-nil-error.
- **v1.8.10:** Multi-provider auth settings + `oauthState` / `loginUser`
  facebook|github|x|linkedin; `OAuthSub` on users.
- **v1.8.9:** relation `Type` forward/backward normalize on upsertConnection.

## Last Updated
2026-08-07

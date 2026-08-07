# open-core — Handoff

- `main` — tag **v1.8.11** pushed (upsert insert-with-`_id`).
- Pinned in engine **v2.4.30** with open_driver **v1.0.14**.
- Drivers still report missing documents inconsistently; only the message
  fallback in `IsDocumentNotFoundErr` covers them. New driver code should wrap
  `ae.ErrDocumentNotFound`. Changing postgres/mysql/mariadb to error instead of
  returning an empty document was deliberately **not** done — other callers rely
  on the empty-doc behaviour.

## Last Updated
2026-08-07

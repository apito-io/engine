# Handoff — open-core 2026-08-09

## Done (uncommitted)

- RegisterUser hook + auth caps + `sdk_bootstrap`
- Resolver gates: login→auth.login, register→auth.register,
  createUser→members.write (+ admin)
- `token.go`: ak_ + cookies=false → apiKeyManager path
- Tests: authz preset, `auth_bootstrap_security_test.go`

## Next

- Restart consumers (engine) with replace
- Confirm tag v1.8.12 after smoke

## Do not

- Narrow synthetic admin globally (deferred)
- Auto-tag without ask

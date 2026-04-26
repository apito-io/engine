// Package sql implements ApitoSystemDB for PostgreSQL, MySQL/MariaDB, and SQLite (including Turso/libsql via the pro wrapper).
//
// Bounded parity audit (SQL vs pro Arango driver, 2026-04): Arango is not a strict superset—several Arango methods are still TODO/panic while SQL implements working variants. Items below are SQL gaps or low-fidelity stubs to revisit only if a call site requires them.
//
// SQL stubs / simplified behavior:
//
//   - SearchResource returns an empty result set (interface_methods.go).
//   - RemoveATeamMemberFromProject / CheckTeamMemberExists use TeamProject queries that may not match project_teams semantics (functions.go); validate before relying on them.
//
// Arango stubs (SQL already implements an equivalent where noted):
//
//   - GetTeams, FindUserOrganizations, AssignTeamToOrganization, RemoveATeamFromOrganization, AssignProjectToOrganization, RemoveProjectFromOrganization, GetProjectTeams, FindUserTeams: panic TODO in arango/misc.go; SQL has working implementations in misc.go / functions.go.
//
// Bootstrap: EnsureSystemBootstrap mirrors Mongo/Arango using database/system/bootstrapmeta; ProSQLSystemDriver embeds SystemSQLDriver and picks this up automatically.

package sql

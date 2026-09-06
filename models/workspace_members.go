package models

import "strings"

const (
	MembershipRoleTeam      = "team"
	MembershipRoleAdmin     = "admin"
	MembershipRoleOwner     = "owner"
	MembershipRoleDeveloper = "developer"
	MembershipRoleViewer    = "viewer"
)

// consoleSectionAliases maps old / informal keys onto GlobalPermissions.
var consoleSectionAliases = map[string]string{
	"content":        "contents",
	"contents":       "contents",
	"model":          "models",
	"models":         "models",
	"files":          "media",
	"storage":        "media",
	"media":          "media",
	"users":          "users",
	"database":       "database",
	"db":             "database",
	"db_explorer":    "database",
	"api":            "api_explorer",
	"api_explorer":   "api_explorer",
	"auth":           "auth",
	"authentication": "auth",
	"logic":          "logic",
	"plugins":        "plugins",
	"plugin":         "plugins",
	"settings":       "settings",
	"webhook":        "webhook",
	"webhooks":       "webhook",
	"api_secrets":    "api_secrets",
	"secrets":        "api_secrets",
	"roles":          "roles",
}

// CanonicalConsoleSection maps a picker/JWT key onto GlobalPermissions, or "".
func CanonicalConsoleSection(raw string) string {
	key := strings.ToLower(strings.TrimSpace(raw))
	if key == "" {
		return ""
	}
	if canon, ok := consoleSectionAliases[key]; ok {
		return canon
	}
	return ""
}

// NormalizeMembershipRole maps invite/edit roles onto owner/admin/developer/viewer.
// Legacy team/editor stay team (collaborator). Project.Roles slugs are not used.
func NormalizeMembershipRole(role string, makeAdmin bool) string {
	if makeAdmin {
		return MembershipRoleAdmin
	}
	r := strings.ToLower(strings.TrimSpace(role))
	switch r {
	case MembershipRoleOwner, "primary_owner":
		return MembershipRoleOwner
	case MembershipRoleAdmin, "project_admin", "make_admin":
		return MembershipRoleAdmin
	case MembershipRoleDeveloper:
		return MembershipRoleDeveloper
	case MembershipRoleViewer:
		return MembershipRoleViewer
	default:
		return MembershipRoleTeam
	}
}

// MembershipPermissions returns console-section keys. Full admin gets GlobalPermissions.
// Unknown / retired keys (teams, addons, extensions, usages) are dropped.
func MembershipPermissions(perms []string, makeAdmin bool) []string {
	if makeAdmin {
		out := make([]string, len(GlobalPermissions))
		copy(out, GlobalPermissions)
		return out
	}
	seen := map[string]struct{}{}
	var out []string
	for _, p := range perms {
		p = CanonicalConsoleSection(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// IsCollaboratorMembershipRole is true for grants that must not count as owned projects.
func IsCollaboratorMembershipRole(role string) bool {
	r := strings.ToLower(strings.TrimSpace(role))
	return r == MembershipRoleTeam || r == "editor" || r == MembershipRoleDeveloper || r == MembershipRoleViewer
}

// IsPrimaryOwnerRole is the legal/commercial owner stored on user_projects.
func IsPrimaryOwnerRole(role string) bool {
	r := strings.ToLower(strings.TrimSpace(role))
	return r == MembershipRoleOwner || r == "primary_owner"
}

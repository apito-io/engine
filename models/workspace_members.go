package models

import "strings"

// WorkspaceMember is one SystemUser with console grants across projects.
type WorkspaceMember struct {
	ID        string                  `json:"id,omitempty"`
	Email     string                  `json:"email,omitempty"`
	FirstName string                  `json:"first_name,omitempty"`
	LastName  string                  `json:"last_name,omitempty"`
	Avatar    string                  `json:"avatar,omitempty"`
	Grants    []*WorkspaceMemberGrant `json:"grants,omitempty"`
}

// WorkspaceMemberGrant is one user_projects row the caller may see or edit.
type WorkspaceMemberGrant struct {
	ProjectID       string   `json:"project_id,omitempty"`
	ProjectName     string   `json:"project_name,omitempty"`
	Role            string   `json:"role,omitempty"`
	Permissions     []string `json:"permissions,omitempty"`
	InviteStatus    string   `json:"invite_status,omitempty"`
	InviteExpiresAt string   `json:"invite_expires_at,omitempty"`
}

// WorkspaceMemberUpsertRequest is invite or edit of grants on selected projects.
type WorkspaceMemberUpsertRequest struct {
	Email                     string
	UserID                    string
	ProjectIDs                []string
	AdministrativePermissions []string
	MakeAdmin                 bool
	// ReplaceExisting, when true, removes caller-managed grants not in ProjectIDs.
	ReplaceExisting bool
}

const (
	MembershipRoleTeam  = "team"
	MembershipRoleAdmin = "admin"
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

// NormalizeMembershipRole returns team or admin. Project.Roles slugs are not used.
func NormalizeMembershipRole(role string, makeAdmin bool) string {
	if makeAdmin {
		return MembershipRoleAdmin
	}
	r := strings.ToLower(strings.TrimSpace(role))
	if r == MembershipRoleAdmin || r == "owner" || r == "project_admin" {
		return MembershipRoleAdmin
	}
	return MembershipRoleTeam
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
	return r == MembershipRoleTeam || r == "editor"
}

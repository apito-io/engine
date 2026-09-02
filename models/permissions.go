package models

// GlobalPermissions is the canonical console-section catalog stored on
// user_projects.permissions. Keys must match console CONSOLE_SECTIONS.
//
// Maps to live project console nav + settings sub-pages (2026). Dropped the
// 2024 leftovers: teams (now workspace /teams), addons, extensions (use
// plugins), usages (billing is /subscriptions).
var GlobalPermissions = []string{
	"contents",
	"models",
	"users",
	"media",
	"database",
	"api_explorer",
	"auth",
	"logic",
	"plugins",
	"settings",
	"webhook",
	"api_secrets",
	"roles",
}

type SchemaBuildPermission struct {
	CanQuery        bool
	CanCreateRecord bool
	CanEditRecord   bool
	CanDeleteRecord bool
}

func BuildPermissions(role string) *SchemaBuildPermission {
	switch role {
	case "admin":
		return &SchemaBuildPermission{
			CanQuery:        true,
			CanCreateRecord: true,
			CanEditRecord:   true,
			CanDeleteRecord: true,
		}
	case "developer":
		return &SchemaBuildPermission{
			CanQuery:        true,
			CanCreateRecord: true,
			CanEditRecord:   true,
			CanDeleteRecord: false,
		}
	case "editor":
		return &SchemaBuildPermission{
			CanQuery:        true,
			CanCreateRecord: true,
			CanEditRecord:   true,
			CanDeleteRecord: false,
		}
	case "public":
		return &SchemaBuildPermission{
			CanQuery:        true,
			CanCreateRecord: false,
			CanEditRecord:   false,
			CanDeleteRecord: false,
		}
	default:
		return nil
	}
}

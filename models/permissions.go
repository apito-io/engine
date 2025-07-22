package models

var GlobalPermissions = []string{"contents", "models", "plugins", "media", "logic", "settings",
	"api_explorer", "usages", "addons", "extensions", "teams", "webhook",
	"api_secrets", "roles"}

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

	return nil
}

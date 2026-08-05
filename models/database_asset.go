package models

// DatabaseAsset describes a physical database that can be announced to the backup coordinator.
// Mirrors backup_system Asset so pro_driver can populate it without importing the backup module.
type DatabaseAsset struct {
	Kind      string // system | project | tenant
	TargetID  string // empty for system; projectID otherwise
	ScopeKey  string // tenant scope when Kind=tenant
	Engine    string // sqlite | libsql | postgresql | ...
	LocalPath string // SQLite-like file path
	DSN       string // networked engines
	CacheKey  string
}

// Asset kind constants (open-core; keep in sync with backup_system/interfaces).
const (
	DatabaseAssetKindSystem  = "system"
	DatabaseAssetKindProject = "project"
	DatabaseAssetKindTenant  = "tenant"
)

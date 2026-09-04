package registry

import (
	"time"

	plugin "github.com/apito-io/engine/services/plugin"
)

// Catalog is the signed registry document.
type Catalog struct {
	SchemaVersion int            `json:"schema_version"`
	GeneratedAt   string         `json:"generated_at"`
	Plugins       []CatalogEntry `json:"plugins"`
}

// CatalogEntry is one reviewed plugin.
type CatalogEntry struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Publisher       string                `json:"publisher"`
	Repository      string                `json:"repository"`
	License         string                `json:"license"`
	Description     string                `json:"description"`
	LongDescription string                `json:"long_description,omitempty"`
	Icon            string                `json:"icon,omitempty"`
	Tags            []string              `json:"tags,omitempty"`
	Category        string                `json:"category"`
	Language        string                `json:"language"`
	Capabilities    []string              `json:"capabilities"`
	EngineSemver    string                `json:"engine_semver"`
	PluginVersion   string                `json:"plugin_version"`
	Trust           string                `json:"trust"`  // official | community
	Status          string                `json:"status"` // available | deprecated | blocked | catalog-stub
	Documentation   string                `json:"documentation_url,omitempty"`
	Runtime         RuntimeContract       `json:"runtime"`
	Releases        []ReleaseAsset        `json:"releases"`
	Approval        *ApprovalMeta         `json:"approval,omitempty"`
	Contributions   *plugin.Contributions `json:"contributions,omitempty"`
}

// RuntimeContract must match repository config.yml.
type RuntimeContract struct {
	ID           string    `json:"id"`
	Version      string    `json:"version"`
	BinaryPath   string    `json:"binary_path"`
	Capabilities []string  `json:"capabilities"`
	Handshake    Handshake `json:"handshake"`
}

// Handshake is the HashiCorp cookie pair.
type Handshake struct {
	ProtocolVersion  int32  `json:"protocol_version"`
	MagicCookieKey   string `json:"magic_cookie_key"`
	MagicCookieValue string `json:"magic_cookie_value"`
}

// ReleaseAsset is one immutable GitHub Release zip.
type ReleaseAsset struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	URL    string `json:"url"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// ApprovalMeta is reviewer provenance.
type ApprovalMeta struct {
	ReviewedBy string `json:"reviewed_by,omitempty"`
	ReviewedAt string `json:"reviewed_at,omitempty"`
}

// InstallRecord is persisted next to the plugin directory.
type InstallRecord struct {
	ID              string    `json:"id"`
	Version         string    `json:"version"`
	CatalogDigest   string    `json:"catalog_digest"`
	ArtifactSHA256  string    `json:"artifact_sha256"`
	OS              string    `json:"os"`
	Arch            string    `json:"arch"`
	Actor           string    `json:"actor"`
	InstalledAt     time.Time `json:"installed_at"`
	State           string    `json:"state"` // installed | rolled_back | uninstalled
	PreviousVersion string    `json:"previous_version,omitempty"`
	SourceURL       string    `json:"source_url,omitempty"`
}

// MergedPlugin is catalog + local install/health.
type MergedPlugin struct {
	CatalogEntry
	Installed         bool   `json:"installed"`
	InstalledVersion  string `json:"installed_version,omitempty"`
	Health            string `json:"health,omitempty"`
	UpdateAvailable   bool   `json:"update_available"`
	Compatible        bool   `json:"compatible"`
	PlatformSupported bool   `json:"platform_supported"`
	BlockedReason     string `json:"blocked_reason,omitempty"`
}

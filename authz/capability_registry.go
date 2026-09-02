package authz

import "strings"

// Canonical capability identifiers.
const (
	CapProjectsRead  = "projects.read"
	CapProjectsWrite = "projects.write"

	CapMembersRead  = "members.read"
	CapMembersWrite = "members.write"

	CapRolesRead  = "roles.read"
	CapRolesWrite = "roles.write"

	CapPlansRead  = "plans.read"
	CapPlansWrite = "plans.write"

	CapTenantsRead   = "tenants.read"
	CapTenantsWrite  = "tenants.write"
	CapTenantsDelete = "tenants.delete"

	CapSchemaRead    = "schema.read"
	CapSchemaWrite   = "schema.write"
	CapSchemaPublish = "schema.publish"

	CapDataRead   = "data.read"
	CapDataWrite  = "data.write"
	CapDataDelete = "data.delete"

	CapRelationsWrite = "relations.write"

	CapFunctionsRead   = "functions.read"
	CapFunctionsWrite  = "functions.write"
	CapFunctionsTest   = "functions.test"
	CapFunctionsDeploy = "functions.deploy"
	CapFunctionsDelete = "functions.delete"

	CapFilesRead   = "files.read"
	CapFilesWrite  = "files.write"
	CapFilesDelete = "files.delete"

	CapPluginsRead   = "plugins.read"
	CapPluginsWrite  = "plugins.write"
	CapPluginsDeploy = "plugins.deploy"

	CapSyncRead  = "sync.read"
	CapSyncWrite = "sync.write"

	CapAuditRead     = "audit.read"
	CapSettingsRead  = "settings.read"
	CapSettingsWrite = "settings.write"

	CapDatabaseRead  = "database.read"
	CapDatabaseWrite = "database.write"

	CapAuthLogin    = "auth.login"
	CapAuthRegister = "auth.register"
)

// Risk tiers for Console danger acknowledgements.
const (
	RiskLow    = "low"
	RiskMedium = "medium"
	RiskHigh   = "high"
	RiskDanger = "danger"
)

// Capability describes one registry entry.
type Capability struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Group       string   `json:"group"`
	Description string   `json:"description"`
	Risk        string   `json:"risk"`
	Implies     []string `json:"implies,omitempty"`
	Danger      bool     `json:"danger,omitempty"`
}

// Preset is a Console-facing capability bundle.
type Preset struct {
	ID           string   `json:"id"`
	Label        string   `json:"label"`
	Description  string   `json:"description"`
	Capabilities []string `json:"capabilities"`
	Danger       bool     `json:"danger,omitempty"`
	DefaultDays  int      `json:"default_days"`
}

var registry = []Capability{
	{ID: CapProjectsRead, Label: "Read projects", Group: "Projects", Risk: RiskLow},
	{ID: CapProjectsWrite, Label: "Write projects", Group: "Projects", Risk: RiskMedium, Implies: []string{CapProjectsRead}},
	{ID: CapMembersRead, Label: "Read members", Group: "Members", Risk: RiskLow},
	{ID: CapMembersWrite, Label: "Write members", Group: "Members", Risk: RiskMedium, Implies: []string{CapMembersRead}},
	{ID: CapRolesRead, Label: "Read roles", Group: "Roles", Risk: RiskLow},
	{ID: CapRolesWrite, Label: "Write roles", Group: "Roles", Risk: RiskHigh, Implies: []string{CapRolesRead}},
	{ID: CapPlansRead, Label: "Read plans", Group: "Plans", Risk: RiskLow},
	{ID: CapPlansWrite, Label: "Write plans", Group: "Plans", Risk: RiskHigh, Implies: []string{CapPlansRead}},
	{ID: CapTenantsRead, Label: "Read tenants", Group: "Tenants", Risk: RiskLow},
	{ID: CapTenantsWrite, Label: "Write tenants", Group: "Tenants", Risk: RiskMedium, Implies: []string{CapTenantsRead}},
	{ID: CapTenantsDelete, Label: "Delete tenants", Group: "Tenants", Risk: RiskDanger, Danger: true, Implies: []string{CapTenantsRead}},
	{ID: CapSchemaRead, Label: "Read schema", Group: "Schema", Risk: RiskLow},
	{ID: CapSchemaWrite, Label: "Write schema", Group: "Schema", Risk: RiskHigh, Implies: []string{CapSchemaRead}},
	{ID: CapSchemaPublish, Label: "Publish schema", Group: "Schema", Risk: RiskDanger, Danger: true, Implies: []string{CapSchemaWrite, CapSchemaRead}},
	{ID: CapDataRead, Label: "Read data", Group: "Data", Risk: RiskLow},
	{ID: CapDataWrite, Label: "Write data", Group: "Data", Risk: RiskMedium, Implies: []string{CapDataRead}},
	{ID: CapDataDelete, Label: "Delete data", Group: "Data", Risk: RiskDanger, Danger: true, Implies: []string{CapDataRead}},
	{ID: CapRelationsWrite, Label: "Write relations", Group: "Data", Risk: RiskMedium},
	{ID: CapFunctionsRead, Label: "Read functions", Group: "Functions", Risk: RiskLow},
	{ID: CapFunctionsWrite, Label: "Write functions", Group: "Functions", Risk: RiskMedium, Implies: []string{CapFunctionsRead}},
	{ID: CapFunctionsTest, Label: "Test functions", Group: "Functions", Risk: RiskMedium, Implies: []string{CapFunctionsRead}},
	{ID: CapFunctionsDeploy, Label: "Deploy functions", Group: "Functions", Risk: RiskDanger, Danger: true, Implies: []string{CapFunctionsWrite, CapFunctionsRead}},
	{ID: CapFunctionsDelete, Label: "Delete functions", Group: "Functions", Risk: RiskDanger, Danger: true, Implies: []string{CapFunctionsRead}},
	{ID: CapFilesRead, Label: "Read files", Group: "Files", Risk: RiskLow},
	{ID: CapFilesWrite, Label: "Write files", Group: "Files", Risk: RiskMedium, Implies: []string{CapFilesRead}},
	{ID: CapFilesDelete, Label: "Delete files", Group: "Files", Risk: RiskHigh, Danger: true, Implies: []string{CapFilesRead}},
	{ID: CapPluginsRead, Label: "Read plugins", Group: "Plugins", Risk: RiskLow},
	{ID: CapPluginsWrite, Label: "Write plugins", Group: "Plugins", Risk: RiskHigh, Implies: []string{CapPluginsRead}},
	{ID: CapPluginsDeploy, Label: "Deploy plugins", Group: "Plugins", Risk: RiskDanger, Danger: true, Implies: []string{CapPluginsWrite, CapPluginsRead}},
	{ID: CapSyncRead, Label: "Sync read", Group: "Sync", Risk: RiskLow},
	{ID: CapSyncWrite, Label: "Sync write", Group: "Sync", Risk: RiskHigh, Implies: []string{CapSyncRead}},
	{ID: CapAuditRead, Label: "Read audit", Group: "Audit", Risk: RiskLow},
	{ID: CapSettingsRead, Label: "Read settings", Group: "Settings", Risk: RiskLow},
	{ID: CapSettingsWrite, Label: "Write settings", Group: "Settings", Risk: RiskHigh, Implies: []string{CapSettingsRead}},
	{ID: CapDatabaseRead, Label: "Read database", Group: "Database", Risk: RiskDanger, Danger: true},
	{ID: CapDatabaseWrite, Label: "Write database", Group: "Database", Risk: RiskDanger, Danger: true, Implies: []string{CapDatabaseRead}},
	{ID: CapAuthLogin, Label: "Login end-users", Group: "Auth", Risk: RiskLow},
	{ID: CapAuthRegister, Label: "Public registerUser", Group: "Auth", Risk: RiskMedium},
}

var presets = []Preset{
	{
		ID:           "full_access",
		Label:        "Full access",
		Description:  "All capabilities. Requires owner/admin and danger acknowledgement.",
		Danger:       true,
		DefaultDays:  30,
		Capabilities: AllCapabilityIDs(),
	},
	{
		ID:          "read_only",
		Label:       "Read-only automation",
		Description: "Read projects, schema, data, functions, files, settings, audit.",
		DefaultDays: 90,
		Capabilities: []string{
			CapProjectsRead, CapMembersRead, CapRolesRead, CapPlansRead, CapTenantsRead,
			CapSchemaRead, CapDataRead, CapFunctionsRead, CapFilesRead,
			CapPluginsRead, CapSyncRead, CapAuditRead, CapSettingsRead,
		},
	},
	{
		ID:          "cli_sync",
		Label:       "CLI sync",
		Description: "Sync read/write plus schema and function surfaces used by apito sync.",
		DefaultDays: 90,
		Capabilities: []string{
			CapProjectsRead, CapSchemaRead, CapSchemaWrite, CapSyncRead, CapSyncWrite,
			CapFunctionsRead, CapFunctionsWrite, CapFunctionsDeploy, CapSettingsRead,
		},
	},
	{
		ID:          "function_deploy",
		Label:       "Function deploy",
		Description: "Read/write/test/deploy edge functions.",
		DefaultDays: 30,
		Capabilities: []string{
			CapProjectsRead, CapFunctionsRead, CapFunctionsWrite, CapFunctionsTest, CapFunctionsDeploy,
		},
	},
	{
		ID:          "data_import_export",
		Label:       "Data import/export",
		Description: "Read and write documents and relations.",
		DefaultDays: 90,
		Capabilities: []string{
			CapProjectsRead, CapDataRead, CapDataWrite, CapRelationsWrite, CapFilesRead, CapFilesWrite,
		},
	},
	{
		ID:          "mcp_assistant",
		Label:       "MCP assistant",
		Description: "Read-only tools for MCP assistants (default).",
		DefaultDays: 90,
		Capabilities: []string{
			CapProjectsRead, CapMembersRead, CapRolesRead, CapPlansRead, CapTenantsRead,
			CapSchemaRead, CapDataRead, CapFunctionsRead, CapFilesRead,
			CapPluginsRead, CapSettingsRead, CapAuditRead,
		},
	},
	{
		ID:          "sdk_bootstrap",
		Label:       "SDK bootstrap",
		Description: "Server/edge bootstrap for login/register/policy reads. Do not embed in mobile or public web.",
		DefaultDays: 90,
		Capabilities: []string{
			CapProjectsRead, CapAuthLogin, CapAuthRegister, CapSettingsRead, CapDataRead,
			// Mobile/SDK Plans UI needs catalog (prices + provider_products) before purchase.
			CapPlansRead,
		},
	},
	{
		ID:          "saas_tenant_ops",
		Label:       "SaaS tenant ops (BFF)",
		Description: "Server-only Protiva/Astro BFF catalog reads+writes (updateTenant, searchTenants). Never ship to browsers.",
		DefaultDays: 90,
		Capabilities: []string{
			CapProjectsRead, CapTenantsRead, CapTenantsWrite, CapSettingsRead, CapDataRead,
			CapAuthLogin, CapAuthRegister,
		},
	},
	{
		ID:           "custom",
		Label:        "Custom",
		Description:  "Pick capabilities individually.",
		DefaultDays:  90,
		Capabilities: nil,
	},
}

var byID map[string]Capability

func init() {
	byID = make(map[string]Capability, len(registry))
	for _, c := range registry {
		byID[c.ID] = c
	}
}

// All returns a copy of the capability registry.
func All() []Capability {
	out := make([]Capability, len(registry))
	copy(out, registry)
	return out
}

// AllCapabilityIDs returns every registered capability id.
func AllCapabilityIDs() []string {
	out := make([]string, 0, len(registry))
	for _, c := range registry {
		out = append(out, c.ID)
	}
	return out
}

// Presets returns Console presets.
func Presets() []Preset {
	out := make([]Preset, len(presets))
	copy(out, presets)
	return out
}

// Get returns a capability by id.
func Get(id string) (Capability, bool) {
	c, ok := byID[strings.TrimSpace(id)]
	return c, ok
}

// ExpandImplies adds implied capabilities transitively.
func ExpandImplies(caps []string) []string {
	set := make(map[string]struct{})
	var walk func(string)
	walk = func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := set[id]; ok {
			return
		}
		set[id] = struct{}{}
		if c, ok := byID[id]; ok {
			for _, imp := range c.Implies {
				walk(imp)
			}
		}
	}
	for _, id := range caps {
		walk(id)
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

// ValidateCapabilities rejects unknown ids. Returns normalized unique list (with implies expanded).
func ValidateCapabilities(caps []string) ([]string, error) {
	for _, id := range caps {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := byID[id]; !ok {
			return nil, ErrUnknownCapability(id)
		}
	}
	return ExpandImplies(caps), nil
}

// HasCapability reports whether granted (already expanded) includes required.
func HasCapability(granted []string, required string) bool {
	required = strings.TrimSpace(required)
	if required == "" {
		return true
	}
	for _, g := range granted {
		if g == required {
			return true
		}
	}
	return false
}

// HasDangerCapability reports whether any capability requires danger acknowledgement.
func HasDangerCapability(caps []string) bool {
	for _, id := range caps {
		if c, ok := byID[id]; ok && c.Danger {
			return true
		}
	}
	return false
}

// ResolvePreset returns capabilities for a preset id. custom returns nil (caller supplies).
func ResolvePreset(presetID string, custom []string) ([]string, error) {
	id := strings.ToLower(strings.TrimSpace(presetID))
	if id == "" || id == "custom" {
		return ValidateCapabilities(custom)
	}
	for _, p := range presets {
		if p.ID == id {
			if len(custom) > 0 {
				merged := append(append([]string{}, p.Capabilities...), custom...)
				return ValidateCapabilities(merged)
			}
			return ValidateCapabilities(p.Capabilities)
		}
	}
	return nil, ErrUnknownPreset(presetID)
}

// ErrUnknownCapability is returned for invalid capability ids.
type unknownCapabilityError struct{ ID string }

func (e unknownCapabilityError) Error() string {
	return "unknown capability: " + e.ID
}

// ErrUnknownCapability constructs the error.
func ErrUnknownCapability(id string) error { return unknownCapabilityError{ID: id} }

type unknownPresetError struct{ ID string }

func (e unknownPresetError) Error() string {
	return "unknown preset: " + e.ID
}

// ErrUnknownPreset constructs the error.
func ErrUnknownPreset(id string) error { return unknownPresetError{ID: id} }

// OperationBinding maps a logical operation to required capability.
type OperationBinding struct {
	Operation  string `json:"operation"`
	Capability string `json:"capability"`
	Surface    string `json:"surface"`
}

// DefaultOperationBindings is the explicit endpoint→capability map (enforce gradually).
func DefaultOperationBindings() []OperationBinding {
	return []OperationBinding{
		{Surface: "system_graphql", Operation: "listProjects", Capability: CapProjectsRead},
		{Surface: "system_graphql", Operation: "createProject", Capability: CapProjectsWrite},
		{Surface: "system_graphql", Operation: "updateProject", Capability: CapProjectsWrite},
		{Surface: "system_graphql", Operation: "deleteProject", Capability: CapProjectsWrite},
		{Surface: "system_graphql", Operation: "listModels", Capability: CapSchemaRead},
		{Surface: "system_graphql", Operation: "upsertModel", Capability: CapSchemaWrite},
		{Surface: "system_graphql", Operation: "publishSchema", Capability: CapSchemaPublish},
		{Surface: "system_graphql", Operation: "listRoles", Capability: CapRolesRead},
		{Surface: "system_graphql", Operation: "upsertRole", Capability: CapRolesWrite},
		{Surface: "system_graphql", Operation: "upsertRoleToProject", Capability: CapRolesWrite},
		{Surface: "system_graphql", Operation: "duplicateRoleInProject", Capability: CapRolesWrite},
		{Surface: "system_graphql", Operation: "deleteRoleFromProject", Capability: CapRolesWrite},
		{Surface: "system_graphql", Operation: "deleteRole", Capability: CapRolesWrite},
		{Surface: "system_graphql", Operation: "getProjectRoles", Capability: CapRolesRead},
		{Surface: "system_graphql", Operation: "listPermissionsAndScopes", Capability: CapRolesRead},
		{Surface: "system_graphql", Operation: "getProjectPlans", Capability: CapPlansRead},
		{Surface: "system_graphql", Operation: "upsertPlanToProject", Capability: CapPlansWrite},
		{Surface: "system_graphql", Operation: "duplicatePlanInProject", Capability: CapPlansWrite},
		{Surface: "system_graphql", Operation: "deletePlanFromProject", Capability: CapPlansWrite},
		{Surface: "system_graphql", Operation: "generateProjectToken", Capability: CapProjectsWrite},
		{Surface: "system_graphql", Operation: "deleteProjectToken", Capability: CapProjectsWrite},
		{Surface: "system_graphql", Operation: "currentProject", Capability: CapProjectsRead},
		{Surface: "system_graphql", Operation: "listMembers", Capability: CapMembersRead},
		{Surface: "system_graphql", Operation: "upsertMember", Capability: CapMembersWrite},
		{Surface: "system_graphql", Operation: "listTenants", Capability: CapTenantsRead},
		{Surface: "system_graphql", Operation: "getTenants", Capability: CapTenantsRead},
		{Surface: "system_graphql", Operation: "searchTenants", Capability: CapTenantsRead},
		{Surface: "system_graphql", Operation: "searchTenantsByDomain", Capability: CapTenantsRead},
		{Surface: "system_graphql", Operation: "upsertTenant", Capability: CapTenantsWrite},
		{Surface: "system_graphql", Operation: "searchPartners", Capability: CapTenantsRead},
		{Surface: "system_graphql", Operation: "createPartner", Capability: CapTenantsWrite},
		{Surface: "system_graphql", Operation: "updatePartner", Capability: CapTenantsWrite},
		{Surface: "system_graphql", Operation: "createTenant", Capability: CapTenantsWrite},
		{Surface: "system_graphql", Operation: "updateTenant", Capability: CapTenantsWrite},
		{Surface: "system_graphql", Operation: "deleteTenant", Capability: CapTenantsDelete},
		{Surface: "system_graphql", Operation: "softDeleteTenant", Capability: CapTenantsDelete},
		{Surface: "system_graphql", Operation: "hardDeleteTenant", Capability: CapTenantsDelete},
		{Surface: "system_graphql", Operation: "restoreTenant", Capability: CapTenantsWrite},
		{Surface: "system_graphql", Operation: "getSettings", Capability: CapSettingsRead},
		{Surface: "system_graphql", Operation: "updateSettings", Capability: CapSettingsWrite},
		{Surface: "system_graphql", Operation: "loginUser", Capability: CapAuthLogin},
		{Surface: "system_graphql", Operation: "registerUser", Capability: CapAuthRegister},
		{Surface: "system_graphql", Operation: "createUser", Capability: CapMembersWrite},
		{Surface: "system_graphql", Operation: "listFunctions", Capability: CapFunctionsRead},
		{Surface: "system_graphql", Operation: "upsertFunction", Capability: CapFunctionsWrite},
		{Surface: "system_graphql", Operation: "testFunction", Capability: CapFunctionsTest},
		{Surface: "system_graphql", Operation: "deployFunction", Capability: CapFunctionsDeploy},
		{Surface: "system_graphql", Operation: "deleteFunction", Capability: CapFunctionsDelete},
		{Surface: "public_graphql", Operation: "queryDocuments", Capability: CapDataRead},
		{Surface: "public_graphql", Operation: "mutateDocuments", Capability: CapDataWrite},
		{Surface: "public_graphql", Operation: "deleteDocuments", Capability: CapDataDelete},
		{Surface: "public_graphql", Operation: "connectRelation", Capability: CapRelationsWrite},
		{Surface: "rest", Operation: "uploadFile", Capability: CapFilesWrite},
		{Surface: "rest", Operation: "listMedia", Capability: CapFilesRead},
		{Surface: "rest", Operation: "deleteMedia", Capability: CapFilesDelete},
		{Surface: "plugin", Operation: "listPlugins", Capability: CapPluginsRead},
		{Surface: "plugin", Operation: "configurePlugin", Capability: CapPluginsWrite},
		{Surface: "plugin", Operation: "deployPlugin", Capability: CapPluginsDeploy},
		{Surface: "sync", Operation: "syncDiff", Capability: CapSyncRead},
		{Surface: "sync", Operation: "syncApply", Capability: CapSyncWrite},
		{Surface: "database", Operation: "databaseRead", Capability: CapDatabaseRead},
		{Surface: "database", Operation: "databaseWrite", Capability: CapDatabaseWrite},
		{Surface: "audit", Operation: "listAudit", Capability: CapAuditRead},
	}
}

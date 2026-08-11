package models

import (
	"github.com/apito-io/types/protobuff"
	"github.com/uptrace/bun"
)

type MetaField struct {
	SourceID string `json:"source_id,omitempty" firestore:"source_id,omitempty" bson:"source_id,omitempty"`

	CreatedAt      string      `json:"created_at,omitempty" firestore:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt      string      `json:"updated_at,omitempty" firestore:"updated_at,omitempty" bson:"updated_at,omitempty"`
	CreatedBy      *SystemUser `json:"created_by,omitempty" firestore:"title,omitempty" bson:"created_by,omitempty"`
	LastModifiedBy *SystemUser `json:"last_modified_by,omitempty" firestore:"created_by,omitempty" bson:"last_modified_by,omitempty"`

	Status string `json:"status,omitempty" firestore:"status,omitempty" bson:"status,omitempty"`
	RootRevisionID string `json:"root_revision_id,omitempty" firestore:"root_revision_id,omitempty" bson:"root_revision_id,omitempty"`
	Revision       bool   `json:"revision,omitempty" firestore:"revision,omitempty" bson:"revision,omitempty"`
	RevisionAt     string `json:"revision_at,omitempty" firestore:"revision_at,omitempty" bson:"revision_at,omitempty"`

	// used in filterAbsentStudent where multiple record is processed but we need to return only attendance id
	ResourceID string `json:"resource_id,omitempty" firestore:"resource_id,omitempty" bson:"resource_id,omitempty"`
}

type SystemUser struct {
	XKey string `json:"_key,omitempty" firestore:"_key,omitempty" bson:"_key,omitempty"`
	ID   string `bun:"type:uuid,pk" json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`

	Secret       string `json:"secret,omitempty" firestore:"secret,omitempty" bson:"secret,omitempty"`
	TempPassword string `json:"temp_password,omitempty" firestore:"temp_password,omitempty" bson:"temp_password,omitempty"`

	FirstName string `json:"first_name,omitempty" firestore:"first_name,omitempty" bson:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty" firestore:"last_name,omitempty" bson:"last_name,omitempty"`
	Role      string `json:"role,omitempty" firestore:"role,omitempty" bson:"role,omitempty"`
	Username  string `json:"username,omitempty" firestore:"username,omitempty" bson:"username,omitempty"`
	Email     string `json:"email,omitempty" firestore:"email,omitempty" bson:"email,omitempty"`
	Avatar    string `json:"avatar,omitempty" firestore:"avatar,omitempty" bson:"avatar,omitempty"`

	CurrentProjectID string `json:"current_project_id,omitempty" firestore:"current_project_id,omitempty" bson:"current_project_id,omitempty"`
	RegisterProvider string `json:"register_provider,omitempty" firestore:"register_provider,omitempty" bson:"register_provider,omitempty"`

	ProjectUser               bool     `json:"project_user,omitempty" firestore:"project_user,omitempty" bson:"project_user,omitempty"`
	AdministrativePermissions []string `json:"administrative_permissions,omitempty" firestore:"email,omitempty" bson:"administrative_permissions,omitempty"`

	ProjectAssignedRole      string   `json:"project_assigned_role,omitempty" bson:"project_assigned_role,omitempty"`
	ProjectAccessPermissions []string `json:"project_access_permissions,omitempty" bson:"project_access_permissions,omitempty"`

	IsAdmin bool `json:"is_admin,omitempty" firestore:"is_admin,omitempty" bson:"is_admin,omitempty"`

	RefreshToken    string `json:"refresh_token,omitempty" firestore:"refresh_token,omitempty" bson:"refresh_token,omitempty"`
	AccessToken     string `json:"access_token,omitempty" firestore:"access_token,omitempty" bson:"access_token,omitempty"`
	ReadOnlyProject bool   `json:"read_only_project,omitempty" firestore:"read_only_project,omitempty" bson:"read_only_project,omitempty"`
	LastLoggedIn    string `json:"last_logged_in,omitempty" firestore:"last_logged_in,omitempty" bson:"last_logged_in,omitempty"`

	CreatedAt string `bun:"type:timestamp,notnull,default:current_timestamp" json:"created_at,omitempty" firestore:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt string `bun:"type:timestamp,notnull" json:"updated_at,omitempty" firestore:"updated_at,omitempty" bson:"updated_at,omitempty"`

	IsPaymentDue bool `json:"is_payment_due,omitempty" firestore:"is_payment_due,omitempty" bson:"is_payment_due,omitempty"`

	// Legacy sync tokens (cli-/sdk-/mcp-). Cleared on apt_ mint; no longer issued or validated.
	SyncTokens []*SyncToken `bun:"type:jsonb,nullzero" json:"sync_tokens,omitempty" firestore:"sync_tokens,omitempty" bson:"sync_tokens,omitempty"`

	// AccessTokens are opaque apt_ automation credentials (hashed secrets only).
	AccessTokens []*AccessTokenRecord `bun:"type:jsonb,nullzero" json:"access_tokens,omitempty" firestore:"access_tokens,omitempty" bson:"access_tokens,omitempty"`

	DefaultTeamID         string        `bun:"default_team_id,type:uuid" json:"default_team_id,omitempty" firestore:"default_team_id,omitempty" bson:"default_team_id,omitempty"`
	DefaultOrganizationID string        `bun:"default_org_id,type:uuid" json:"default_org_id,omitempty" firestore:"default_org_id,omitempty" bson:"default_org_id,omitempty"`
	DefaultTeam           *Team         `bun:"rel:belongs-to,join:default_team_id=id" json:"default_team,omitempty" firestore:"default_team,omitempty" bson:"default_team,omitempty"`
	DefaultOrganization   *Organization `bun:"rel:belongs-to,join:default_org_id=id" json:"default_organization,omitempty" firestore:"default_organization,omitempty" bson:"default_organization,omitempty"`

	Projects     []*Project      `bun:"m2m:user_projects,join:User=Project" json:"projects,omitempty" firestore:"projects,omitempty" bson:"projects,omitempty"`
	Teams        []*Team         `bun:"m2m:user_teams,join:User=Team" json:"teams,omitempty" firestore:"teams,omitempty" bson:"teams,omitempty"`
	Organization []*Organization `bun:"rel:has-many,join:id=user_id" json:"organization,omitempty" firestore:"organization,omitempty" bson:"organization,omitempty"`

	IsActive bool `json:"is_active,omitempty" firestore:"is_active,omitempty" bson:"is_active,omitempty"`
}

type ProjectSettings struct {
	bun.BaseModel `bun:"table:project_settings"`

	ProjectID             string   `bun:"type:uuid,pk" json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"_id,omitempty"`
	Locals                []string `bun:"type:json,nullzero" json:"locals,omitempty" firestore:"locals,omitempty" bson:"locals,omitempty"`
	SystemGraphqlHooks    bool     `bun:",notnull" json:"system_graphql_hooks,omitempty" firestore:"system_graphql_hooks,omitempty" bson:"system_graphql_hooks,omitempty"`
	EnableRevisionHistory bool     `bun:",notnull" json:"enable_revision_history,omitempty" firestore:"revision_history,omitempty" bson:"enable_revision_history,omitempty"`

	DefaultStoragePlugin  string `bun:",nullzero" json:"default_storage_plugin,omitempty" firestore:"default_storage_plugin,omitempty" bson:"default_storage_plugin,omitempty"`
	DefaultFunctionPlugin string `bun:",nullzero" json:"default_function_plugin,omitempty" firestore:"default_function_plugin,omitempty" bson:"default_function_plugin,omitempty"`

	DefaultLocale string `bun:",nullzero" json:"default_locale,omitempty" firestore:"default_locale,omitempty" bson:"default_locale,omitempty"`

	// IdleTenantRetentionDays is the inactivity window (days) for idle tenant listing / auto soft-delete.
	// Default and minimum are 90. Zero/unset is treated as 90 when reading.
	IdleTenantRetentionDays int `bun:"idle_tenant_retention_days,notnull,default:90" json:"idle_tenant_retention_days,omitempty" firestore:"idle_tenant_retention_days,omitempty" bson:"idle_tenant_retention_days,omitempty"`
	// AutoSoftDeleteIdleTenants enables daily soft-delete of free-tier idle tenants (never hard-delete).
	AutoSoftDeleteIdleTenants bool `bun:"auto_soft_delete_idle_tenants,notnull,default:false" json:"auto_soft_delete_idle_tenants,omitempty" firestore:"auto_soft_delete_idle_tenants,omitempty" bson:"auto_soft_delete_idle_tenants,omitempty"`
}

// MinIdleTenantRetentionDays is the lowest allowed idle_tenant_retention_days value.
const MinIdleTenantRetentionDays = 90

// EffectiveIdleTenantRetentionDays returns retention days (default/clamp floor 90).
func EffectiveIdleTenantRetentionDays(s *ProjectSettings) int {
	if s == nil || s.IdleTenantRetentionDays <= 0 {
		return MinIdleTenantRetentionDays
	}
	if s.IdleTenantRetentionDays < MinIdleTenantRetentionDays {
		return MinIdleTenantRetentionDays
	}
	return s.IdleTenantRetentionDays
}

type SavedPluginDetails struct {
	ProjectID      string                         `bun:"type:uuid,notnull" json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	ID             string                         `json:"id,omitempty" firestore:"id,omitempty" bson:"id,omitempty"`
	EnvVars        []*protobuff.EnvVariable       `json:"env_vars,omitempty" firestore:"env_vars,omitempty" bson:"env_vars,omitempty"`
	ActivateStatus protobuff.PluginActivateStatus `json:"activate_status,omitempty" firestore:"activate_status,omitempty" bson:"activate_status,omitempty"`
	LoadStatus     protobuff.PluginLoadStatus     `json:"load_status,omitempty" firestore:"load_status,omitempty" bson:"load_status,omitempty"`
	Enable         bool                           `json:"enable,omitempty" firestore:"enable,omitempty" bson:"enable,omitempty"`
}

// Project user project
type Project struct {
	XKey string `json:"_key,omitempty" firestore:"_key,omitempty" bson:"_key,omitempty"`
	ID   string `bun:"type:uuid,pk" json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`

	Name        string                `json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	Description string                `json:"description,omitempty" firestore:"description,omitempty" bson:"description,omitempty"`
	// ProjectIcon is an optional branding URL (or media file URL) for invoices / receipts.
	ProjectIcon string                `json:"project_icon,omitempty" firestore:"project_icon,omitempty" bson:"project_icon,omitempty"`
	Schema      *ProjectSchema        `bun:"rel:belongs-to,join:id=project_id" json:"schema,omitempty" firestore:"schema,omitempty" bson:"schema,omitempty"`
	CreatedAt   string                `json:"created_at,omitempty" firestore:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt   string                `json:"updated_at,omitempty" firestore:"updated_at,omitempty" bson:"updated_at,omitempty"`
	ExpireAt    string                `json:"expire_at,omitempty" firestore:"expire_at,omitempty" bson:"expire_at,omitempty"`
	Plugins     []*SavedPluginDetails `bun:"rel:has-many" json:"plugins,omitempty" firestore:"plugins,omitempty" bson:"plugins,omitempty"`
	Settings    *ProjectSettings      `bun:"rel:belongs-to,join:id=project_id" json:"settings,omitempty"  firestore:"settings,omitempty" bson:"settings,omitempty"`

	Tokens []*ProjectToken `bun:"rel:has-many" json:"tokens,omitempty" firestore:"tokens,omitempty" bson:"tokens,omitempty"`

	Roles      map[string]*Role   `bun:"type:jsonb" json:"roles,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3" firestore:"roles,omitempty" bson:"roles,omitempty"`
	// Plans are project-defined permission ceilings keyed by slug (free/paid/…).
	// Assigned per tenant via pro_tenants.plan_tier. Nil/empty is treated as fully permissive.
	// Only free is system_generated; other tiers are operator-owned customs.
	Plans map[string]*Plan `bun:"type:jsonb" json:"plans,omitempty" firestore:"plans,omitempty" bson:"plans,omitempty"`
	// PlanSeedOmissions lists default seed slugs the operator removed so EnsureProjectPlansSeeds
	// does not recreate them. Free is never omitted. Legacy paid_* seeds are no longer re-seeded.
	PlanSeedOmissions []string `bun:"type:jsonb" json:"plan_seed_omissions,omitempty" firestore:"plan_seed_omissions,omitempty" bson:"plan_seed_omissions,omitempty"`
	Driver     *DriverCredentials `bun:"rel:belongs-to,join:id=project_id" json:"driver,omitempty"  firestore:"driver,omitempty" bson:"driver,omitempty"`
	TempBanned bool               `json:"temp_banned,omitempty" firestore:"temp_banned,omitempty" bson:"temp_banned,omitempty"`

	ProjectTemplate string `json:"project_template,omitempty" firestore:"project_template,omitempty" bson:"project_template,omitempty"`

	Teams          []*Team       `bun:"m2m:team_projects,join:Project=Team" json:"teams,omitempty"  firestore:"teams,omitempty" bson:"teams,omitempty"`
	Users          []*SystemUser `bun:"m2m:user_projects,join:Project=User" json:"users,omitempty"  firestore:"users,omitempty" bson:"users,omitempty"`
	OrganizationID string        `json:"organization_id,omitempty" firestore:"organization_id,omitempty" bson:"organization_id,omitempty"`
	Organization   *Organization `bun:"rel:belongs-to,join:organization_id=id" json:"organization,omitempty" firestore:"organization,omitempty" bson:"organization,omitempty"`

	SystemMessages []*SystemMessage `bun:"rel:has-many" json:"system_messages,omitempty" firestore:"system_messages,omitempty" bson:"system_messages,omitempty"`
	Workspaces     []*Workspace     `bun:"rel:has-many" json:"workspaces,omitempty" firestore:"workspaces,omitempty" bson:"workspaces,omitempty"`

	// for sync
	SyncedProperty *SyncProject `bun:"rel:belongs-to,join:id=project_id" json:"synced_property,omitempty" firestore:"synced_property,omitempty" bson:"synced_property,omitempty"`

	ProjectSecretKey string `json:"project_secret_key,omitempty" firestore:"project_secret_key,omitempty" bson:"project_secret_key,omitempty"`

	AuthenticationSettings *AuthenticationSettings `json:"authentication_settings,omitempty" firestore:"authentication_settings,omitempty" bson:"authentication_settings,omitempty"`
	StorageSettings        *StorageSettings        `json:"storage_settings,omitempty" firestore:"storage_settings,omitempty" bson:"storage_settings,omitempty"`
}



type SyncProject struct {
	ProjectID                string `bun:"type:uuid,pk" json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"_id,omitempty"`
	SyncedTokenUsed          string `json:"synced_token_used,omitempty" firestore:"synced_token_used,omitempty" bson:"synced_token_used,omitempty"`
	LocalProjectID           string `json:"local_project_id,omitempty" firestore:"local_project_id,omitempty" bson:"local_project_id,omitempty"`
	MergeWithExistingProject bool   `json:"merge_with_existing_project,omitempty" firestore:"merge_with_existing_project,omitempty" bson:"merge_with_existing_project,omitempty"`
	LastSyncedAt             string `json:"last_synced_at,omitempty" firestore:"last_synced_at,omitempty" bson:"last_synced_at,omitempty"`
}

type ProjectWithRoles struct {
	User        *SystemUser `json:"user,omitempty" firestore:"user,omitempty" bson:"user,omitempty"`
	Project     *Project    `json:"project,omitempty" firestore:"project" bson:"project,omitempty"`
	Role        string      `json:"role,omitempty" firestore:"role,omitempty" bson:"role,omitempty"`
	Permissions []string    `json:"permissions,omitempty" firestore:"permissions,omitempty" bson:"permissions,omitempty"`
}

package interfaces

import (
	"context"

	"github.com/apito-io/engine/models"
)

// ApitoSystemDB is the main interface for the system database operations
type ApitoSystemDB interface {

	// RunMigration runs the database migrations
	RunMigration(ctx context.Context) error

	// EnsureSystemBootstrap creates idempotent first-run data for this engine (default admin,
	// default org/team/project where applicable). Safe to call on every startup after RunMigration.
	EnsureSystemBootstrap(ctx context.Context) error

	// Project-related functions CreateProject creates a new project
	CreateProject(ctx context.Context, userId string, project *models.Project) (*models.Project, error)
	// UpdateProject updates a project
	UpdateProject(ctx context.Context, project *models.Project, replace bool) error
	// PersistProjectModelTypes reconciles normalized model_types rows (SQL): deletes rows not in models,
	// then upserts each entry. Pass an empty slice to remove all models for the project.
	// Document stores (MongoDB, BBolt, ArangoDB) no-op; schema is saved with UpdateProject.
	PersistProjectModelTypes(ctx context.Context, projectID string, schemaModels []*models.ModelType) error
	// UpsertModelType inserts or updates a single model_types row (SQL). Does not delete orphans or touch other models.
	// Document stores merge this model into the project document schema.
	UpsertModelType(ctx context.Context, projectID string, m *models.ModelType) error
	// DeleteModelType removes one model_types row (SQL) or one model from embedded schema (document stores).
	DeleteModelType(ctx context.Context, projectID, modelName string) error
	// TouchProjectUpdatedAt sets projects.updated_at without persisting schema (for granular schema edits).
	TouchProjectUpdatedAt(ctx context.Context, projectID string) error
	// GetProjects retrieves multiple projects by their IDs
	GetProjects(ctx context.Context, keys []string) ([]*models.Project, error)
	// GetProject retrieves a project by its ID
	GetProject(ctx context.Context, id string) (*models.Project, error)
	// CheckProjectName checks if a project name already exists
	CheckProjectName(ctx context.Context, name string) error
	// SearchProjects lists all the projects for a given user
	SearchProjects(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Project], error)
	// FindUserProjects lists all the projects for a given user
	FindUserProjects(ctx context.Context, userId string) ([]*models.Project, error)
	// FindUserProjectsWithRoles lists all the projects for a given user with their roles
	FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error)
	// DeleteProjectFromSystem deletes a project from the system
	DeleteProjectFromSystem(ctx context.Context, projectId string) error
	// SaveProjectAuthenticationSettings persists authentication_settings on the project row/document.
	SaveProjectAuthenticationSettings(ctx context.Context, projectID string, auth *models.AuthenticationSettings) error
	// SaveProjectStorageSettings persists storage_settings on the project row/document.
	SaveProjectStorageSettings(ctx context.Context, projectID string, storage *models.StorageSettings) error

	// Project application end-users (table: project_users). Not SystemUser (console operators).
	CreateUser(ctx context.Context, user *models.User) (*models.User, error)
	GetUser(ctx context.Context, projectID, userID string) (*models.User, error)
	UpdateUser(ctx context.Context, user *models.User) error
	DeleteUser(ctx context.Context, projectID, userID string) error
	SearchProjectUsers(ctx context.Context, projectID, q string, limit, offset int) ([]*models.User, int, error)
	CountProjectUsersByRole(ctx context.Context, projectID string) (map[string]int, error)
	GetUserByUsername(ctx context.Context, projectID, username string) (*models.User, error)
	ListUsersByEmail(ctx context.Context, projectID, email string) ([]*models.User, error)
	ListUsersByPhone(ctx context.Context, projectID, phone string) ([]*models.User, error)
	ListUsersByGoogleSub(ctx context.Context, projectID, googleSub string) ([]*models.User, error)

	// AddATeamMemberToProject adds a team member to a project
	AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error
	// RemoveATeamMemberFromProject removes a team member from a project
	RemoveATeamMemberFromProject(ctx context.Context, projectId string, userId string) error

	// SearchFunctions lists all the cloud functions for a given project
	SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error)

	// User-related functions
	// GetSystemUser retrieves a system user by their ID
	GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error)
	// GetSystemUserByEmail retrieves a system user by their email
	GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error)
	// GetSystemUsers retrieves multiple system users by their IDs
	GetSystemUsers(ctx context.Context, keys []string) ([]*models.SystemUser, error)
	// CreateSystemUser creates a new system user
	CreateSystemUser(ctx context.Context, user *models.SystemUser) (*models.SystemUser, error)
	// UpdateSystemUser updates a system user's profile
	UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error
	// SearchSystemUsers searches for system users based on a filter
	SearchSystemUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error)
	// CheckProjectWithRoles checks if a user belongs to a project
	CheckProjectWithRoles(ctx context.Context, userId, projectId string) (*models.ProjectWithRoles, error)
	// AddSystemUserMetaInfo adds metadata to a system user
	// this is deprecated, and replaced by dataloader resolver
	// AddSystemUserMetaInfo(ctx context.Context, doc *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error)
	// SearchResource searches for system users based on a filter
	SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error)

	// Organization-related functions
	GetTeams(ctx context.Context, userId string) ([]*models.Team, error)
	// GetOrganizations retrieves multiple user organizations by their IDs
	GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error)
	// FindOrganizationAdmin retrieves an organization admin by their ID
	FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error)
	// FindUserOrganizations retrieves all the organizations for a given user
	FindUserOrganizations(ctx context.Context, userId string) ([]*models.Organization, error)
	// CreateOrganization creates a new organization
	CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error)
	// AssignTeamToOrganization adds a team to an organization
	AssignTeamToOrganization(ctx context.Context, orgId, userId, teamId string) error
	// RemoveATeamFromOrganization removes a team from an organization
	RemoveATeamFromOrganization(ctx context.Context, orgId, userId, teamId string) error
	// AssignProjectToOrganization assigns a project to an organization
	AssignProjectToOrganization(ctx context.Context, orgId, userId, projectId string) error
	// RemoveProjectFromOrganization removes a project from an organization
	RemoveProjectFromOrganization(ctx context.Context, orgId, userId, projectId string) error

	// Team-related functions
	// GetProjectTeams retrieves a team member from a project
	GetProjectTeams(ctx context.Context, projectId string) (*models.Team, error)
	// FindUserTeams retrieves all the teams for a given user
	GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error)
	// CreateTeam creates a new team
	CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error)
	// AddTeamMetaInfo adds metadata to a team
	AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error)

	// Audit log-related functions
	// SaveAuditLog saves an audit log
	SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error
	// SearchAuditLogs retrieves audit logs based on a filter
	SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error)

	// Webhook-related functions
	// SearchWebHooks lists all the webhooks for a given project
	SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error)
	// GetWebHook retrieves a webhook by its ID for a given project
	GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error)
	// DeleteWebhook deletes a webhook by its ID for a given project
	DeleteWebhook(ctx context.Context, projectId, hookId string) error
	// AddWebhookToProject adds a webhook to a project
	AddWebhookToProject(ctx context.Context, doc *models.Webhook) (*models.Webhook, error)

	// Raw data-related functions
	// SaveRawData saves raw data related to payments
	SaveRawData(ctx context.Context, collection string, data map[string]interface{}) error

	// Token-related functions
	// CheckTokenBlacklisted checks if a token is blacklisted
	CheckTokenBlacklisted(ctx context.Context, tokenId string) error
	// BlacklistAToken blacklists a token
	BlacklistAToken(ctx context.Context, token map[string]interface{}) error
}

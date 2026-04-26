package sql

import (
	"github.com/apito-io/engine/models"
	"github.com/apito-io/types/protobuff"
	"github.com/uptrace/bun"
)

// RegisterSystemSQLSchemaModels registers m2m join types (user_projects, etc.) on this *bun.DB.
// Required for any query touching models with bun m2m tags — not only during RunMigration.
// Call once per opened system DB connection (idempotent).
func RegisterSystemSQLSchemaModels(orm *bun.DB) {
	orm.RegisterModel(
		(*models.UserProject)(nil),
		(*models.TeamProject)(nil),
		(*models.UserTeam)(nil),
		(*models.UserOrganization)(nil),
		(*models.OrganizationTeam)(nil),
		(*models.ProjectSettings)(nil),
	)
}

// systemSQLSchemaModels returns the full ordered list of tables for RunMigration (single source of truth).
func systemSQLSchemaModels() []interface{} {
	return []interface{}{
		(*models.SystemUser)(nil),
		(*models.Team)(nil),
		(*models.Organization)(nil),
		(*models.Project)(nil),
		(*models.ProjectSettings)(nil),
		(*models.ProjectSchema)(nil),
		(*protobuff.PluginDetails)(nil),
		(*models.ProjectToken)(nil),
		(*models.DriverCredentials)(nil),
		(*models.SystemMessage)(nil),
		(*models.ModelType)(nil),
		(*models.ApitoFunction)(nil),
		(*models.UserProject)(nil),
		(*models.UserTeam)(nil),
		(*models.TeamProject)(nil),
		(*models.UserOrganization)(nil),
		(*models.OrganizationTeam)(nil),
		(*organizationProjectRow)(nil),
		(*projectTeamRow)(nil),
		(*tokenBlacklistRow)(nil),
		(*rawDataRow)(nil),
		(*userMetadataRow)(nil),
		(*teamMetadataRow)(nil),
		(*models.AuditLogs)(nil),
		(*models.Webhook)(nil),
	}
}

package sqlcommon

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

// SystemSQLSchemaModels returns the full ordered list of tables for RunMigration.
func SystemSQLSchemaModels() []interface{} {
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
		(*OrganizationProjectRow)(nil),
		(*ProjectTeamRow)(nil),
		(*TokenBlacklistRow)(nil),
		(*RawDataRow)(nil),
		(*UserMetadataRow)(nil),
		(*TeamMetadataRow)(nil),
		(*models.AuditLogs)(nil),
		(*models.Webhook)(nil),
		(*models.User)(nil),
		(*models.SchemaOperation)(nil),
	}
}

package sqlcommon

import "github.com/uptrace/bun"

// Bun models used only for RunMigration CreateTable.

type TokenBlacklistRow struct {
	bun.BaseModel `bun:"table:token_blacklist"`

	TokenID string `bun:"token_id,pk,type:text"`
	Data    string `bun:"data,type:text"`
}

type RawDataRow struct {
	bun.BaseModel `bun:"table:raw_data"`

	ID         string `bun:"id,pk,type:uuid"`
	Collection string `bun:"collection,type:text"`
	Data       string `bun:"data,type:text"`
}

type UserMetadataRow struct {
	bun.BaseModel `bun:"table:user_metadata"`

	ID   string `bun:"id,pk,type:uuid"`
	Data string `bun:"data,type:text"`
}

type TeamMetadataRow struct {
	bun.BaseModel `bun:"table:team_metadata"`

	ID   string `bun:"id,pk,type:uuid"`
	Data string `bun:"data,type:text"`
}

type OrganizationProjectRow struct {
	bun.BaseModel `bun:"table:organization_projects"`

	OrganizationID string `bun:"organization_id,type:uuid,pk"`
	ProjectID      string `bun:"project_id,type:uuid,pk"`
	AssignedBy     string `bun:"assigned_by,type:uuid,nullzero"`
	AssignedAt     string `bun:"assigned_at,type:timestamp,nullzero"`
}

type ProjectTeamRow struct {
	bun.BaseModel `bun:"table:project_teams"`

	ProjectID string `bun:"project_id,type:uuid,pk"`
	UserID    string `bun:"user_id,type:uuid,pk"`
}

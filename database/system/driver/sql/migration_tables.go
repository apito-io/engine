package sql

import "github.com/uptrace/bun"

// Bun models used only for RunMigration CreateTable (table names match TableExpr / inserts).

type tokenBlacklistRow struct {
	bun.BaseModel `bun:"table:token_blacklist"`

	TokenID string `bun:"token_id,pk,type:text"`
	Data    string `bun:"data,type:text"`
}

type rawDataRow struct {
	bun.BaseModel `bun:"table:raw_data"`

	ID         string `bun:"id,pk,type:uuid"`
	Collection string `bun:"collection,type:text"`
	Data       string `bun:"data,type:text"`
}

type userMetadataRow struct {
	bun.BaseModel `bun:"table:user_metadata"`

	ID   string `bun:"id,pk,type:uuid"`
	Data string `bun:"data,type:text"`
}

type teamMetadataRow struct {
	bun.BaseModel `bun:"table:team_metadata"`

	ID   string `bun:"id,pk,type:uuid"`
	Data string `bun:"data,type:text"`
}

type organizationProjectRow struct {
	bun.BaseModel `bun:"table:organization_projects"`

	OrganizationID string `bun:"organization_id,type:uuid,pk"`
	ProjectID      string `bun:"project_id,type:uuid,pk"`
	AssignedBy     string `bun:"assigned_by,type:uuid,nullzero"`
	AssignedAt     string `bun:"assigned_at,type:timestamp,nullzero"`
}

type projectTeamRow struct {
	bun.BaseModel `bun:"table:project_teams"`

	ProjectID string `bun:"project_id,type:uuid,pk"`
	UserID    string `bun:"user_id,type:uuid,pk"`
}

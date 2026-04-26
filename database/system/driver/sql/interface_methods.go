package sql

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// FindUserProjectsWithRoles implements interfaces.ApitoSystemDB.
func (p *SystemSQLDriver) FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	return p.GetProjectWithRolesAndPermission(ctx, userId)
}

// GetTeams implements interfaces.ApitoSystemDB.
func (p *SystemSQLDriver) GetTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	return p.FindUserTeams(ctx, userId)
}

// GetTeamsMembers implements interfaces.ApitoSystemDB.
func (p *SystemSQLDriver) GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	return p.ListTeams(ctx, projectId)
}

// SearchResource implements interfaces.ApitoSystemDB.
func (p *SystemSQLDriver) SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error) {
	return &models.SearchResponse[any]{
		Results: []*any{},
	}, nil
}

// FindOrganizationAdmin implements interfaces.ApitoSystemDB.
func (p *SystemSQLDriver) FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error) {
	user := &models.SystemUser{}
	err := p.ORM.NewSelect().
		Model(user).
		Join("JOIN user_organizations uo ON uo.user_id = system_user.id").
		Where("uo.organization_id = ? AND uo.role = ?", orgId, "admin").
		Limit(1).
		Scan(ctx)

	return user, err
}

// SaveAuditLog implements interfaces.ApitoSystemDB.
func (p *SystemSQLDriver) SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error {
	if auditLog.ID == "" {
		auditLog.ID = utility.NewID()
	}

	_, err := p.ORM.NewInsert().
		Model(auditLog).
		Exec(ctx)

	return err
}

// SearchAuditLogs implements interfaces.ApitoSystemDB.
func (p *SystemSQLDriver) SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error) {
	var logs []*models.AuditLogs

	query := p.ORM.NewSelect().Model(&logs)

	if param.UserID != "" {
		query = query.Where("user_id = ?", param.UserID)
	}
	if param.ProjectID != "" {
		query = query.Where("project_id = ?", param.ProjectID)
	}

	err := query.Order("created_at DESC").Limit(100).Scan(ctx)
	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[models.AuditLogs]{
		Results: logs,
	}, nil
}

// GetOrganizations implements interfaces.ApitoSystemDB.
func (p *SystemSQLDriver) GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error) {
	var organizations []*models.Organization

	err := p.ORM.NewSelect().
		Model(&organizations).
		Join("JOIN user_organizations uo ON uo.organization_id = organization.id").
		Where("uo.user_id = ?", userId).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[models.Organization]{
		Results: organizations,
	}, nil
}

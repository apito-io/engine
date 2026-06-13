package postgres

import (
	"github.com/apito-io/engine/database/system/sqlcommon"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/uptrace/bun"
)

// GetProjects retrieves multiple projects by their IDs using SQL
func (d *Driver) GetProjects(ctx context.Context, keys []string) ([]*models.Project, error) {
	var projects []*models.Project
	err := d.ORM.NewSelect().
		Model(&projects).
		Relation("Driver").
		Relation("Settings").
		Relation("Schema.Models").
		Where("id IN (?)", bun.In(keys)).
		Scan(ctx)

	return projects, err
}

// AddATeamMemberToProject adds a team member to a project using SQL
func (d *Driver) AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error {
	// Create user-project relation with role
	_, err := d.ORM.NewInsert().
		Model(&map[string]interface{}{
			"user_id":     req.UserID,
			"project_id":  req.ProjectID,
			"role":        req.Role,
			"permissions": req.Permissions,
		}).
		TableExpr("user_projects").
		Exec(ctx)

	if err != nil {
		return err
	}

	// Also add to project_teams table
	_, err = d.ORM.NewInsert().
		Model(&map[string]interface{}{
			"user_id":    req.UserID,
			"project_id": req.ProjectID,
		}).
		TableExpr("project_teams").
		Exec(ctx)

	return err
}

// GetSystemUsers retrieves multiple system users by their IDs using SQL
func (d *Driver) GetSystemUsers(ctx context.Context, keys []string) ([]*models.SystemUser, error) {
	var users []*models.SystemUser
	err := d.ORM.NewSelect().
		Model(&users).
		Where("id IN (?)", bun.In(keys)).
		Scan(ctx)

	return users, err
}

// FindUserOrganizations retrieves all organizations for a given user using SQL
func (d *Driver) FindUserOrganizations(ctx context.Context, userId string) ([]*models.Organization, error) {
	var organizations []*models.Organization

	err := d.ORM.NewSelect().
		Model(&organizations).
		Join("JOIN user_organizations uo ON uo.organization_id = organization.id").
		Where("uo.user_id = ?", userId).
		Scan(ctx)

	return organizations, err
}

// CreateOrganization creates a new organization using SQL
func (d *Driver) CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error) {
	if org.ID == "" {
		org.ID = utility.NewID()
	}

	_, err := d.ORM.NewInsert().
		Model(org).
		Exec(ctx)

	return org, err
}

// AssignTeamToOrganization assigns a team to an organization using SQL
func (d *Driver) AssignTeamToOrganization(ctx context.Context, orgId, userId, teamId string) error {
	_, err := d.ORM.NewInsert().
		Model(&map[string]interface{}{
			"organization_id": orgId,
			"team_id":         teamId,
			"assigned_by":     userId,
			"assigned_at":     time.Now(),
		}).
		TableExpr("organization_teams").
		Exec(ctx)

	return err
}

// RemoveATeamFromOrganization removes a team from an organization using SQL
func (d *Driver) RemoveATeamFromOrganization(ctx context.Context, orgId, userId, teamId string) error {
	_, err := d.ORM.NewDelete().
		Model((*models.OrganizationTeam)(nil)).
		Where("organization_id = ? AND team_id = ?", orgId, teamId).
		Exec(ctx)

	return err
}

// AssignProjectToOrganization assigns a project to an organization using SQL
func (d *Driver) AssignProjectToOrganization(ctx context.Context, orgId, userId, projectId string) error {
	_, err := d.ORM.NewInsert().
		Model(&map[string]interface{}{
			"organization_id": orgId,
			"project_id":      projectId,
			"assigned_by":     userId,
			"assigned_at":     time.Now(),
		}).
		TableExpr("organization_projects").
		Exec(ctx)

	return err
}

// RemoveProjectFromOrganization removes a project from an organization using SQL
func (d *Driver) RemoveProjectFromOrganization(ctx context.Context, orgId, userId, projectId string) error {
	_, err := d.ORM.NewDelete().
		Model((*sqlcommon.OrganizationProjectRow)(nil)).
		Where("organization_id = ? AND project_id = ?", orgId, projectId).
		Exec(ctx)

	return err
}

// GetProjectTeams retrieves team information for a project using SQL
func (d *Driver) GetProjectTeams(ctx context.Context, projectId string) (*models.Team, error) {
	team := &models.Team{}
	err := d.ORM.NewSelect().
		Model(team).
		Join("JOIN team_projects tp ON tp.team_id = team.id").
		Where("tp.project_id = ?", projectId).
		Limit(1).
		Scan(ctx)

	return team, err
}

// FindUserTeams retrieves all teams for a given user using SQL
func (d *Driver) FindUserTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	var teams []*models.Team

	err := d.ORM.NewSelect().
		Model(&teams).
		Join("JOIN user_teams ut ON ut.team_id = team.id").
		Where("ut.user_id = ?", userId).
		Scan(ctx)

	return teams, err
}

// CreateTeam creates a new team using SQL
func (d *Driver) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	if team.ID == "" {
		team.ID = utility.NewID()
	}

	_, err := d.ORM.NewInsert().
		Model(team).
		Exec(ctx)

	if err != nil {
		return nil, err
	}

	// Create user-team relations for each user
	for _, user := range team.Users {
		_, err = d.ORM.NewInsert().
			Model(&map[string]interface{}{
				"user_id": user.ID,
				"team_id": team.ID,
			}).
			TableExpr("user_teams").
			Exec(ctx)

		if err != nil {
			return nil, err
		}
	}

	return team, nil
}

// FindUserProjects retrieves all projects for a given user using SQL
func (d *Driver) FindUserProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	var projects []*models.Project

	err := d.ORM.NewSelect().
		Model(&projects).
		Relation("Driver").
		Join("JOIN user_projects up ON up.project_id = project.id").
		Where("up.user_id = ?", userId).
		Scan(ctx)

	return projects, err
}

// CheckProjectWithRoles checks if a user belongs to a project and returns roles/permissions using SQL
func (d *Driver) CheckProjectWithRoles(ctx context.Context, userId, projectId string) (*models.ProjectWithRoles, error) {
	if projectId == "" {
		return nil, fmt.Errorf("project id is empty")
	}

	// Get the project
	project := &models.Project{}
	err := d.ORM.NewSelect().
		Model(project).
		Relation("Driver").
		Relation("Settings").
		Relation("Schema.Models").
		Where("project.id = ?", projectId).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// Get the user
	user := &models.SystemUser{}
	err = d.ORM.NewSelect().
		Model(user).
		Where("id = ?", userId).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// Get user-project relation with role and permissions
	var relation map[string]interface{}
	err = d.ORM.NewSelect().
		Model(&relation).
		TableExpr("user_projects").
		Where("user_id = ? AND project_id = ?", userId, projectId).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	role, _ := relation["role"].(string)
	permissions, _ := relation["permissions"].(string)

	var permList []string
	if permissions != "" {
		json.Unmarshal([]byte(permissions), &permList)
	}

	return &models.ProjectWithRoles{
		User:        user,
		Project:     project,
		Role:        role,
		Permissions: permList,
	}, nil
}

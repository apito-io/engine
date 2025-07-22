package sql

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// GetProject retrieves a project by ID using SQL
func (p *PostgreSQLDriver) GetProject(ctx context.Context, id string) (*models.Project, error) {
	project := &models.Project{}
	err := p.ORM.NewSelect().
		Model(project).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return project, nil
}

// GetSystemUser retrieves a system user by ID using SQL
func (p *PostgreSQLDriver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
	user := &models.SystemUser{}
	err := p.ORM.NewSelect().
		Model(user).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetSystemUserByEmail retrieves a system user by email using SQL
func (p *PostgreSQLDriver) GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error) {
	user := &models.SystemUser{}
	err := p.ORM.NewSelect().
		Model(user).
		Where("email = ?", email).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// CheckProjectName checks if a project name already exists using SQL
func (p *PostgreSQLDriver) CheckProjectName(ctx context.Context, name string) error {
	exists, err := p.ORM.NewSelect().
		Model((*models.Project)(nil)).
		Where("name = ?", name).
		Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("project already exists with this name")
	}
	return nil
}

// SearchProjects searches for projects based on common system parameters using SQL
func (p *PostgreSQLDriver) SearchProjects(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Project], error) {
	var projects []*models.Project

	query := p.ORM.NewSelect().Model(&projects)

	// Add filters based on parameters
	if param.UserID != "" {
		query = query.Join("JOIN user_projects up ON up.project_id = project.id").
			Where("up.user_id = ?", param.UserID)
	}

	// Add pagination (can be implemented based on ResolveParams if needed)
	// For now, using default limits
	query = query.Limit(100) // Default limit

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[models.Project]{
		Results: projects,
	}, nil
}

// GetProjectWithRolesAndPermission retrieves projects with roles and permissions for a user using SQL
func (p *PostgreSQLDriver) GetProjectWithRolesAndPermission(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	var projectWithRoles []*models.ProjectWithRoles

	err := p.ORM.NewSelect().
		Model(&projectWithRoles).
		Join("JOIN user_projects up ON up.project_id = project.id").
		Where("up.user_id = ?", userId).
		Scan(ctx)

	return projectWithRoles, err
}

// ListAllProjects lists all projects for a user (with admin check) using SQL
func (p *PostgreSQLDriver) ListAllProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	// Check if user is admin
	user, err := p.GetSystemUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	if !user.IsSuperAdmin {
		return nil, fmt.Errorf("not allowed")
	}

	var projects []*models.Project
	err = p.ORM.NewSelect().
		Model(&projects).
		Order("created_at DESC").
		Scan(ctx)

	return projects, err
}

// ListAllUsers lists all system users using SQL
func (p *PostgreSQLDriver) ListAllUsers(ctx context.Context) ([]*models.SystemUser, error) {
	var users []*models.SystemUser
	err := p.ORM.NewSelect().
		Model(&users).
		Order("created_at DESC").
		Scan(ctx)

	return users, err
}

// ListTeams lists team members for a project using SQL
func (p *PostgreSQLDriver) ListTeams(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	var users []*models.SystemUser

	err := p.ORM.NewSelect().
		Model(&users).
		Join("JOIN project_teams pt ON pt.user_id = system_user.id").
		Where("pt.project_id = ?", projectId).
		Scan(ctx)

	return users, err
}

// SearchFunctions searches for cloud functions in a project using SQL
func (p *PostgreSQLDriver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	project, err := p.GetProject(ctx, param.ProjectID)
	if err != nil {
		return nil, err
	}

	if project.Schema == nil {
		return nil, fmt.Errorf("schema is nil")
	}

	return &models.SearchResponse[models.ApitoFunction]{
		Results: project.Schema.Functions,
	}, nil
}

// SearchWebHooks searches for webhooks in a project using SQL
func (p *PostgreSQLDriver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	var hooks []*models.Webhook

	err := p.ORM.NewSelect().
		Model(&hooks).
		Where("project_id = ?", param.ProjectID).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[models.Webhook]{
		Results: hooks,
	}, nil
}

// GetWebHook retrieves a specific webhook by ID using SQL
func (p *PostgreSQLDriver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	webhook := &models.Webhook{}
	err := p.ORM.NewSelect().
		Model(webhook).
		Where("project_id = ? AND id = ?", projectId, hookId).
		Scan(ctx)

	return webhook, err
}

// DeleteWebhook deletes a webhook using SQL
func (p *PostgreSQLDriver) DeleteWebhook(ctx context.Context, projectId, hookId string) error {
	_, err := p.ORM.NewDelete().
		Model((*models.Webhook)(nil)).
		Where("project_id = ? AND id = ?", projectId, hookId).
		Exec(ctx)

	return err
}

// SearchUsers searches for system users based on parameters using SQL
func (p *PostgreSQLDriver) SearchUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	var users []*models.SystemUser

	query := p.ORM.NewSelect().Model(&users)

	// Add pagination (can be implemented based on ResolveParams if needed)
	// For now, using default limits
	query = query.Limit(100) // Default limit

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}

	return &models.SearchResponse[models.SystemUser]{
		Results: users,
	}, nil
}

// AddSystemUserMetaInfo adds metadata to a system user using SQL
func (p *PostgreSQLDriver) AddSystemUserMetaInfo(ctx context.Context, doc *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	// Create a metadata table entry
	metadata := map[string]interface{}{
		"id":   doc.ID,
		"data": doc,
	}

	jsonData, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	_, err = p.ORM.NewInsert().
		Model(&map[string]interface{}{
			"id":   doc.ID,
			"data": string(jsonData),
		}).
		TableExpr("user_metadata").
		Exec(ctx)

	return doc, err
}

// AddTeamMetaInfo adds metadata to team members using SQL
func (p *PostgreSQLDriver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	for _, doc := range docs {
		jsonData, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}

		_, err = p.ORM.NewInsert().
			Model(&map[string]interface{}{
				"id":   doc.ID,
				"data": string(jsonData),
			}).
			TableExpr("team_metadata").
			Exec(ctx)

		if err != nil {
			return nil, err
		}
	}

	return docs, nil
}

// RemoveATeamMemberFromProject removes a team member from a project using SQL
func (p *PostgreSQLDriver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberID string) error {
	_, err := p.ORM.NewDelete().
		Model((*models.TeamProject)(nil)).
		Where("project_id = ? AND team_id IN (SELECT id FROM teams WHERE user_id = ?)", projectId, memberID).
		Exec(ctx)

	return err
}

// CheckTeamMemberExists checks if a team member exists in a project using SQL
func (p *PostgreSQLDriver) CheckTeamMemberExists(ctx context.Context, projectId string, memberID string) error {
	exists, err := p.ORM.NewSelect().
		Model((*models.TeamProject)(nil)).
		Where("project_id = ? AND team_id IN (SELECT id FROM teams WHERE user_id = ?)", projectId, memberID).
		Exists(ctx)

	if err != nil {
		return err
	}

	if !exists {
		return fmt.Errorf("team member not found")
	}

	return nil
}

// CreateProject creates a new project using SQL
func (p *PostgreSQLDriver) CreateProject(ctx context.Context, userId string, project *models.Project) (*models.Project, error) {
	if project.ID == "" {
		project.ID = uuid.New().String()
	}

	project.CreatedAt = time.Now().Format(time.RFC3339)
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	_, err := p.ORM.NewInsert().
		Model(project).
		Exec(ctx)

	if err != nil {
		return nil, err
	}

	// Create user-project relation
	_, err = p.ORM.NewInsert().
		Model(&map[string]interface{}{
			"user_id":     userId,
			"project_id":  project.ID,
			"role":        "owner",
			"permissions": `["read", "write", "admin"]`,
		}).
		TableExpr("user_projects").
		Exec(ctx)

	return project, err
}

// CreateSystemUser creates a new system user using SQL
func (p *PostgreSQLDriver) CreateSystemUser(ctx context.Context, user *models.SystemUser) (*models.SystemUser, error) {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	user.CreatedAt = time.Now().Format(time.RFC3339)
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	_, err := p.ORM.NewInsert().
		Model(user).
		Exec(ctx)

	return user, err
}

// UpdateSystemUser updates a system user using SQL
func (p *PostgreSQLDriver) UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error {
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	if replace {
		_, err := p.ORM.NewUpdate().
			Model(user).
			Where("id = ?", user.ID).
			Exec(ctx)
		return err
	} else {
		_, err := p.ORM.NewUpdate().
			Model(user).
			Where("id = ?", user.ID).
			ExcludeColumn("id", "created_at").
			Exec(ctx)
		return err
	}
}

// UpdateProject updates a project using SQL
func (p *PostgreSQLDriver) UpdateProject(ctx context.Context, project *models.Project, replace bool) error {
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	if replace {
		_, err := p.ORM.NewUpdate().
			Model(project).
			Where("id = ?", project.ID).
			Exec(ctx)
		return err
	} else {
		_, err := p.ORM.NewUpdate().
			Model(project).
			Where("id = ?", project.ID).
			ExcludeColumn("id", "created_at").
			Exec(ctx)
		return err
	}
}

// CheckTokenBlacklisted checks if a token is blacklisted using SQL
func (p *PostgreSQLDriver) CheckTokenBlacklisted(ctx context.Context, tokenId string) error {
	exists, err := p.ORM.NewSelect().
		Model((*map[string]interface{})(nil)).
		TableExpr("token_blacklist").
		Where("token_id = ?", tokenId).
		Exists(ctx)

	if err != nil {
		return err
	}

	if exists {
		return fmt.Errorf("token is blacklisted")
	}

	return nil
}

// BlacklistAToken adds a token to the blacklist using SQL
func (p *PostgreSQLDriver) BlacklistAToken(ctx context.Context, token map[string]interface{}) error {
	tokenId, exists := token["jti"].(string)
	if !exists {
		return fmt.Errorf("token ID not found")
	}

	jsonData, err := json.Marshal(token)
	if err != nil {
		return err
	}

	_, err = p.ORM.NewInsert().
		Model(&map[string]interface{}{
			"token_id": tokenId,
			"data":     string(jsonData),
		}).
		TableExpr("token_blacklist").
		Exec(ctx)

	return err
}

// DeleteProjectFromSystem deletes a project and all related data using SQL
func (p *PostgreSQLDriver) DeleteProjectFromSystem(ctx context.Context, projectId string) error {
	// Use a transaction to ensure all deletions succeed or fail together
	return p.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Delete user-project relations
		_, err := tx.NewDelete().
			Model((*map[string]interface{})(nil)).
			TableExpr("user_projects").
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Delete project team relations
		_, err = tx.NewDelete().
			Model((*models.TeamProject)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Delete webhooks
		_, err = tx.NewDelete().
			Model((*models.Webhook)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Delete the project
		_, err = tx.NewDelete().
			Model((*models.Project)(nil)).
			Where("id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		return nil
	})
}

// AddWebhookToProject adds a webhook to a project using SQL
func (p *PostgreSQLDriver) AddWebhookToProject(ctx context.Context, doc *models.Webhook) (*models.Webhook, error) {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	_, err := p.ORM.NewInsert().
		Model(doc).
		Exec(ctx)

	return doc, err
}

// SaveRawData saves raw data using SQL for payment-related operations
func (p *PostgreSQLDriver) SaveRawData(ctx context.Context, collection string, data map[string]interface{}) error {
	id := uuid.New().String()
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = p.ORM.NewInsert().
		Model(&map[string]interface{}{
			"id":         id,
			"collection": collection,
			"data":       string(jsonData),
		}).
		TableExpr("raw_data").
		Exec(ctx)

	return err
}

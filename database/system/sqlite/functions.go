package sqlite

import (
	"github.com/apito-io/engine/database/system/sqlcommon"
	"context"
	"encoding/json"
	"fmt"
	"time"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/database/system/driverdefaults"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/uptrace/bun"
)

// GetProject retrieves a project by ID using SQL
func (d *Driver) GetProject(ctx context.Context, id string) (*models.Project, error) {
	project := &models.Project{}
	err := d.ORM.NewSelect().
		Model(project).
		Relation("Driver").
		Relation("Settings").
		Relation("Schema.Models").
		Relation("Tokens").
		Where("project.id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	if err := hydrateProjectSettingsFromDB(ctx, d.ORM, project); err != nil {
		return nil, err
	}
	return project, nil
}

// GetSystemUser retrieves a system user by ID using SQL
func (d *Driver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
	user := &models.SystemUser{}
	err := d.ORM.NewSelect().
		Model(user).
		Where("id = ?", id).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetSystemUserByEmail retrieves a system user by email using SQL
func (d *Driver) GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error) {
	user := &models.SystemUser{}
	err := d.ORM.NewSelect().
		Model(user).
		Where("email = ?", email).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// CheckProjectName checks if a project name already exists using SQL
func (d *Driver) CheckProjectName(ctx context.Context, name string) error {
	exists, err := d.ORM.NewSelect().
		Model((*models.Project)(nil)).
		Where("name = ?", name).
		Exists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("%w", ae.ErrProjectNameTaken)
	}
	return nil
}

// SearchProjects searches for projects based on common system parameters using SQL
func (d *Driver) SearchProjects(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Project], error) {
	var projects []*models.Project

	query := d.ORM.NewSelect().Model(&projects).Relation("Driver").Relation("Settings").Relation("Schema.Models")

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
func (d *Driver) GetProjectWithRolesAndPermission(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	// ProjectWithRoles is a DTO (not a table). Do not Scan into []*ProjectWithRoles — Bun would use table "project_with_roles".
	// Load projects via FindUserProjects, then membership rows from user_projects only (avoids Bun walking Project relations on scan).
	user, err := d.GetSystemUser(ctx, userId)
	if err != nil {
		return nil, err
	}
	projects, err := d.FindUserProjects(ctx, userId)
	if err != nil {
		return nil, err
	}

	var jrows []struct {
		ProjectID   string `bun:"project_id"`
		Role        string `bun:"role"`
		Permissions string `bun:"permissions"`
	}
	err = d.ORM.NewSelect().
		ColumnExpr("project_id").
		ColumnExpr("role").
		ColumnExpr("permissions").
		TableExpr("user_projects").
		Where("user_id = ?", userId).
		Scan(ctx, &jrows)
	if err != nil {
		return nil, err
	}
	byProject := make(map[string]struct {
		Role        string
		Permissions string
	}, len(jrows))
	for i := range jrows {
		byProject[jrows[i].ProjectID] = struct {
			Role        string
			Permissions string
		}{Role: jrows[i].Role, Permissions: jrows[i].Permissions}
	}

	out := make([]*models.ProjectWithRoles, 0, len(projects))
	for _, pr := range projects {
		if pr == nil {
			continue
		}
		j := byProject[pr.ID]
		var permList []string
		if j.Permissions != "" {
			_ = json.Unmarshal([]byte(j.Permissions), &permList)
		}
		proj := *pr
		out = append(out, &models.ProjectWithRoles{
			User:        user,
			Project:     &proj,
			Role:        j.Role,
			Permissions: permList,
		})
	}
	return out, nil
}

// ListAllProjects lists all projects for a user (with admin check) using SQL
func (d *Driver) ListAllProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	// Check if user is admin
	user, err := d.GetSystemUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	if !user.IsAdmin {
		return nil, fmt.Errorf("not allowed")
	}

	var projects []*models.Project
	err = d.ORM.NewSelect().
		Model(&projects).
		Relation("Driver").
		Relation("Settings").
		Relation("Schema.Models").
		Order("created_at DESC").
		Scan(ctx)

	return projects, err
}

// ListAllUsers lists all system users using SQL
func (d *Driver) ListAllUsers(ctx context.Context) ([]*models.SystemUser, error) {
	var users []*models.SystemUser
	err := d.ORM.NewSelect().
		Model(&users).
		Order("created_at DESC").
		Scan(ctx)

	return users, err
}

// ListTeams lists team members for a project using SQL
func (d *Driver) ListTeams(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	var users []*models.SystemUser

	err := d.ORM.NewSelect().
		Model(&users).
		Join("JOIN project_teams pt ON pt.user_id = system_user.id").
		Where("pt.project_id = ?", projectId).
		Scan(ctx)

	return users, err
}

// SearchFunctions searches for cloud functions in a project using SQL
func (d *Driver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	project, err := d.GetProject(ctx, param.ProjectID)
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
func (d *Driver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	var hooks []*models.Webhook

	err := d.ORM.NewSelect().
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
func (d *Driver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	webhook := &models.Webhook{}
	err := d.ORM.NewSelect().
		Model(webhook).
		Where("project_id = ? AND id = ?", projectId, hookId).
		Scan(ctx)

	return webhook, err
}

// DeleteWebhook deletes a webhook using SQL
func (d *Driver) DeleteWebhook(ctx context.Context, projectId, hookId string) error {
	_, err := d.ORM.NewDelete().
		Model((*models.Webhook)(nil)).
		Where("project_id = ? AND id = ?", projectId, hookId).
		Exec(ctx)

	return err
}

// SearchSystemUsers searches for system users based on parameters using SQL
func (d *Driver) SearchSystemUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	var users []*models.SystemUser

	query := d.ORM.NewSelect().Model(&users)

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
func (d *Driver) AddSystemUserMetaInfo(ctx context.Context, doc *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	// Create a metadata table entry
	metadata := map[string]interface{}{
		"id":   doc.ID,
		"data": doc,
	}

	jsonData, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	_, err = d.ORM.NewInsert().
		Model(&map[string]interface{}{
			"id":   doc.ID,
			"data": string(jsonData),
		}).
		TableExpr("user_metadata").
		Exec(ctx)

	return doc, err
}

// AddTeamMetaInfo adds metadata to team members using SQL
func (d *Driver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	for _, doc := range docs {
		jsonData, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}

		_, err = d.ORM.NewInsert().
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
func (d *Driver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberID string) error {
	_, err := d.ORM.NewDelete().
		Model((*models.TeamProject)(nil)).
		Where("project_id = ? AND team_id IN (SELECT id FROM teams WHERE user_id = ?)", projectId, memberID).
		Exec(ctx)

	return err
}

// CheckTeamMemberExists checks if a team member exists in a project using SQL
func (d *Driver) CheckTeamMemberExists(ctx context.Context, projectId string, memberID string) error {
	exists, err := d.ORM.NewSelect().
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
func (d *Driver) CreateProject(ctx context.Context, userId string, project *models.Project) (*models.Project, error) {
	if project.ID == "" {
		project.ID = utility.NewID()
	}

	if project.Driver == nil {
		project.Driver = driverdefaults.OSSBootstrapProjectDriver(d.Conf, project.ID)
		if project.Driver == nil {
			return nil, fmt.Errorf("CreateProject: driver credentials are required for project %s", project.ID)
		}
	} else if project.Driver.ProjectID == "" {
		project.Driver.ProjectID = project.ID
	}

	project.CreatedAt = time.Now().Format(time.RFC3339)
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	driverToPersist := project.Driver
	project.Driver = nil

	err := d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewInsert().
			Model(project).
			Exec(ctx); err != nil {
			return err
		}

		project.Driver = driverToPersist
		if project.Driver != nil {
			if _, err := tx.NewInsert().Model(project.Driver).Exec(ctx); err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
				return err
			}
		}

		// Create user-project relation
		// Match Arango (user_project_edges) and Mongo (user_projects): creator role is "admin",
		// which must exist as a key in project.roles for BuildSystemParam / public schema permissions.
		if _, err := tx.NewInsert().
			Model(&map[string]interface{}{
				"user_id":     userId,
				"project_id":  project.ID,
				"role":        "admin",
				"permissions": `["read", "write", "admin"]`,
			}).
			TableExpr("user_projects").
			Exec(ctx); err != nil {
			return err
		}

		if err := d.ensureProjectSchemaRowTx(ctx, tx, project.ID); err != nil {
			return err
		}
		if err := d.syncProjectSchemaModelsTx(ctx, tx, project); err != nil {
			return err
		}
		if err := d.syncProjectTokensTx(ctx, tx, project); err != nil {
			return err
		}
		return d.syncProjectSettingsTx(ctx, tx, project)
	})
	if err != nil {
		return nil, err
	}

	return project, nil
}

// CreateSystemUser creates a new system user using SQL
func (d *Driver) CreateSystemUser(ctx context.Context, user *models.SystemUser) (*models.SystemUser, error) {
	if user.ID == "" {
		user.ID = utility.NewID()
	}

	user.CreatedAt = time.Now().Format(time.RFC3339)
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	_, err := d.ORM.NewInsert().
		Model(user).
		Exec(ctx)

	return user, err
}

// UpdateSystemUser updates a system user using SQL
func (d *Driver) UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error {
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	if replace {
		_, err := d.ORM.NewUpdate().
			Model(user).
			Where("id = ?", user.ID).
			Exec(ctx)
		return err
	} else {
		_, err := d.ORM.NewUpdate().
			Model(user).
			Where("id = ?", user.ID).
			ExcludeColumn("id", "created_at").
			Exec(ctx)
		return err
	}
}

// UpdateProject updates a project using SQL
func (d *Driver) UpdateProject(ctx context.Context, project *models.Project, replace bool) error {
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		var err error
		if replace {
			_, err = tx.NewUpdate().
				Model(project).
				Where("id = ?", project.ID).
				Exec(ctx)
		} else {
			_, err = tx.NewUpdate().
				Model(project).
				Where("id = ?", project.ID).
				ExcludeColumn("id", "created_at").
				Exec(ctx)
		}
		if err != nil {
			return err
		}
		if err = d.syncProjectSchemaModelsTx(ctx, tx, project); err != nil {
			return err
		}
		if err = d.syncProjectTokensTx(ctx, tx, project); err != nil {
			return err
		}
		if project.Settings != nil {
			return d.syncProjectSettingsTx(ctx, tx, project)
		}
		return nil
	})
}

func (d *Driver) ensureProjectSchemaRowTx(ctx context.Context, tx bun.Tx, projectID string) error {
	if projectID == "" {
		return nil
	}
	exists, err := tx.NewSelect().
		Model((*models.ProjectSchema)(nil)).
		Where("project_id = ?", projectID).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("ensure project_schema: %w", err)
	}
	if exists {
		return nil
	}
	row := &models.ProjectSchema{ProjectID: projectID}
	if _, err := tx.NewInsert().Model(row).Exec(ctx); err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
		return fmt.Errorf("insert project_schemas: %w", err)
	}
	return nil
}

// syncProjectSchemaModels persists ProjectSchema and ModelType rows (ORM relations are not written by Update).
func (d *Driver) syncProjectSchemaModels(ctx context.Context, project *models.Project) error {
	if project == nil || project.Schema == nil {
		return nil
	}
	return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return d.syncProjectSchemaModelsTx(ctx, tx, project)
	})
}

// syncProjectSchemaModelsTx is the transactional implementation (used by CreateProject/UpdateProject in the same tx).
func (d *Driver) syncProjectSchemaModelsTx(ctx context.Context, tx bun.Tx, project *models.Project) error {
	if project == nil || project.Schema == nil {
		return nil
	}
	schema := project.Schema
	schema.ProjectID = project.ID

	schemaExists, err := tx.NewSelect().
		Model((*models.ProjectSchema)(nil)).
		Where("project_id = ?", project.ID).
		Exists(ctx)
	if err != nil {
		return err
	}
	if !schemaExists {
		row := &models.ProjectSchema{
			ProjectID:           project.ID,
			NamingSchemaVersion: schema.NamingSchemaVersion,
		}
		if _, err := tx.NewInsert().Model(row).Exec(ctx); err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
			return err
		}
	} else if schema.NamingSchemaVersion != 0 {
		ps := &models.ProjectSchema{ProjectID: project.ID, NamingSchemaVersion: schema.NamingSchemaVersion}
		if _, err := tx.NewUpdate().
			Model(ps).
			Column("naming_schema_version").
			Where("project_id = ?", project.ID).
			Exec(ctx); err != nil {
			return err
		}
	}

	if schema.Models != nil {
		return d.persistProjectModelTypesTx(ctx, tx, project.ID, schema.Models)
	}
	return nil
}

// PersistProjectModelTypes implements interfaces.ApitoSystemDB: deletes model_types rows for projectID
// whose name is not in schemaModels, then inserts or updates each model row. schemaModels nil means no-op (reserved).
// An empty slice deletes all model_types for the project.
func (d *Driver) PersistProjectModelTypes(ctx context.Context, projectID string, schemaModels []*models.ModelType) error {
	if projectID == "" || schemaModels == nil {
		return nil
	}
	return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return d.persistProjectModelTypesTx(ctx, tx, projectID, schemaModels)
	})
}

func (d *Driver) persistProjectModelTypesTx(ctx context.Context, tx bun.Tx, projectID string, schemaModels []*models.ModelType) error {
	var keepNames []string
	for _, m := range schemaModels {
		if m == nil || m.Name == "" {
			continue
		}
		keepNames = append(keepNames, m.Name)
	}

	var err error
	if len(keepNames) == 0 {
		_, err = tx.NewDelete().
			Model((*models.ModelType)(nil)).
			Where("project_id = ?", projectID).
			Exec(ctx)
	} else {
		_, err = tx.NewDelete().
			Model((*models.ModelType)(nil)).
			Where("project_id = ?", projectID).
			Where("name NOT IN (?)", bun.In(keepNames)).
			Exec(ctx)
	}
	if err != nil {
		return err
	}

	for _, m := range schemaModels {
		if m == nil || m.Name == "" {
			continue
		}
		row := *m
		row.ProjectID = projectID
		_, err := tx.NewInsert().Model(&row).Exec(ctx)
		if err != nil {
			if !sqlcommon.IsSQLUniqueViolation(err) {
				return err
			}
			if _, err = tx.NewUpdate().
				Model(&row).
				Where("project_id = ? AND name = ?", row.ProjectID, row.Name).
				ExcludeColumn("project_id", "name").
				Exec(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

// UpsertModelType implements interfaces.ApitoSystemDB — single model_types row, no orphan reconciliation.
func (d *Driver) UpsertModelType(ctx context.Context, projectID string, m *models.ModelType) error {
	if projectID == "" || m == nil || m.Name == "" {
		return nil
	}
	row := *m
	row.ProjectID = projectID
	_, err := d.ORM.NewInsert().Model(&row).Exec(ctx)
	if err != nil {
		if !sqlcommon.IsSQLUniqueViolation(err) {
			return err
		}
		if _, err = d.ORM.NewUpdate().
			Model(&row).
			Where("project_id = ? AND name = ?", row.ProjectID, row.Name).
			ExcludeColumn("project_id", "name").
			Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

// DeleteModelType implements interfaces.ApitoSystemDB — delete one model_types row.
func (d *Driver) DeleteModelType(ctx context.Context, projectID, modelName string) error {
	if projectID == "" || modelName == "" {
		return nil
	}
	_, err := d.ORM.NewDelete().
		Model((*models.ModelType)(nil)).
		Where("project_id = ? AND name = ?", projectID, modelName).
		Exec(ctx)
	return err
}

// TouchProjectUpdatedAt implements interfaces.ApitoSystemDB.
func (d *Driver) TouchProjectUpdatedAt(ctx context.Context, projectID string) error {
	if projectID == "" {
		return nil
	}
	// RFC3339Nano so granular touches differ within the same calendar second (RFC3339 truncates seconds).
	ts := time.Now().Format(time.RFC3339Nano)
	proj := &models.Project{
		ID:        projectID,
		UpdatedAt: ts,
	}
	_, err := d.ORM.NewUpdate().
		Model(proj).
		Column("updated_at").
		WherePK().
		Exec(ctx)
	return err
}

// syncProjectTokens persists project_tokens (Bun does not insert has-many on UpdateProject).
func (d *Driver) syncProjectTokens(ctx context.Context, project *models.Project) error {
	if project == nil {
		return nil
	}
	return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return d.syncProjectTokensTx(ctx, tx, project)
	})
}

func (d *Driver) syncProjectTokensTx(ctx context.Context, tx bun.Tx, project *models.Project) error {
	if project == nil {
		return nil
	}

	if _, err := tx.NewDelete().
		Model((*models.ProjectToken)(nil)).
		Where("project_id = ?", project.ID).
		Exec(ctx); err != nil {
		return err
	}

	for _, token := range project.Tokens {
		if token == nil {
			continue
		}
		row := *token
		row.ProjectID = project.ID
		if _, err := tx.NewInsert().Model(&row).Exec(ctx); err != nil {
			if !sqlcommon.IsSQLUniqueViolation(err) {
				return err
			}
		}
	}
	return nil
}

// syncProjectSettings persists project_settings (Bun does not insert belongs-to on project insert/update).
func (d *Driver) syncProjectSettings(ctx context.Context, project *models.Project) error {
	if project == nil {
		return nil
	}
	return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return d.syncProjectSettingsTx(ctx, tx, project)
	})
}

func (d *Driver) syncProjectSettingsTx(ctx context.Context, tx bun.Tx, project *models.Project) error {
	if project == nil {
		return nil
	}
	st := project.Settings
	if st == nil {
		st = &models.ProjectSettings{Locals: []string{"en"}}
		project.Settings = st
	} else if len(st.Locals) == 0 {
		st.Locals = []string{"en"}
	}
	st.ProjectID = project.ID

	row := *st
	row.ProjectID = project.ID

	exists, err := tx.NewSelect().
		Model((*models.ProjectSettings)(nil)).
		Where("project_id = ?", project.ID).
		Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		_, err = tx.NewInsert().Model(&row).Exec(ctx)
		return err
	}
	_, err = tx.NewUpdate().
		Model(&row).
		Where("project_id = ?", project.ID).
		ExcludeColumn("project_id").
		Exec(ctx)
	return err
}

// CheckTokenBlacklisted checks if a token is blacklisted using SQL
func (d *Driver) CheckTokenBlacklisted(ctx context.Context, tokenId string) error {
	exists, err := d.ORM.NewSelect().
		Model((*sqlcommon.TokenBlacklistRow)(nil)).
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
func (d *Driver) BlacklistAToken(ctx context.Context, token map[string]interface{}) error {
	tokenId, exists := token["jti"].(string)
	if !exists {
		return fmt.Errorf("token ID not found")
	}

	jsonData, err := json.Marshal(token)
	if err != nil {
		return err
	}

	_, err = d.ORM.NewInsert().
		Model(&map[string]interface{}{
			"token_id": tokenId,
			"data":     string(jsonData),
		}).
		TableExpr("token_blacklist").
		Exec(ctx)

	return err
}

// DeleteProjectFromSystem deletes a project and all related data using SQL
func (d *Driver) DeleteProjectFromSystem(ctx context.Context, projectId string) error {
	// Use a transaction to ensure all deletions succeed or fail together
	return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Join / satellite tables first (FK-safe order). Do not use Model((*map[string]interface{})(nil)) — bun rejects it.

		_, err := tx.NewDelete().
			Model((*models.UserProject)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.NewDelete().
			Model((*sqlcommon.ProjectTeamRow)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.NewDelete().
			Model((*models.TeamProject)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.NewDelete().
			Model((*sqlcommon.OrganizationProjectRow)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.NewDelete().
			Model((*models.ModelType)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.NewDelete().
			Model((*models.ApitoFunction)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.NewDelete().
			Model((*models.ProjectSchema)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.NewDelete().
			Model((*models.ProjectSettings)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.NewDelete().
			Model((*models.DriverCredentials)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.NewDelete().
			Model((*models.AuditLogs)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		_, err = tx.NewDelete().
			Model((*models.Webhook)(nil)).
			Where("project_id = ?", projectId).
			Exec(ctx)
		if err != nil {
			return err
		}

		// Tables without a dedicated bun model / composite layout — raw DELETE.
		for _, q := range []string{
			`DELETE FROM project_tokens WHERE project_id = ?`,
			`DELETE FROM system_messages WHERE project_id = ?`,
		} {
			if _, err = tx.ExecContext(ctx, q, projectId); err != nil {
				return err
			}
		}

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
func (d *Driver) AddWebhookToProject(ctx context.Context, doc *models.Webhook) (*models.Webhook, error) {
	if doc.ID == "" {
		doc.ID = utility.NewID()
	}

	_, err := d.ORM.NewInsert().
		Model(doc).
		Exec(ctx)

	return doc, err
}

// SaveRawData saves raw data using SQL for payment-related operations
func (d *Driver) SaveRawData(ctx context.Context, collection string, data map[string]interface{}) error {
	id := utility.NewID()
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	_, err = d.ORM.NewInsert().
		Model(&map[string]interface{}{
			"id":         id,
			"collection": collection,
			"data":       string(jsonData),
		}).
		TableExpr("raw_data").
		Exec(ctx)

	return err
}

package sql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/apito-io/engine/database/system/bootstrapmeta"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

func (p *SystemSQLDriver) ensureBootstrapAdmin(ctx context.Context) error {
	u, err := p.GetSystemUserByEmail(ctx, bootstrapmeta.AdminEmail)
	if err != nil && !bootstrapmeta.IsUserLookupMiss(err) {
		return fmt.Errorf("bootstrap: check admin user: %w", err)
	}
	if u != nil {
		return nil
	}

	hash, err := bootstrapmeta.HashDefaultAdminPassword()
	if err != nil {
		return fmt.Errorf("bootstrap: hash password: %w", err)
	}

	id := utility.NewID()
	now := utility.GetCurrentTime()
	user := &models.SystemUser{
		XKey:                      id,
		ID:                        id,
		Email:                     bootstrapmeta.AdminEmail,
		Username:                  utility.NewID(),
		FirstName:                 bootstrapmeta.AdminFirstName,
		LastName:                  bootstrapmeta.AdminLastName,
		Secret:                    string(hash),
		IsAdmin:                   true,
		AdministrativePermissions: []string{"all"},
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}

	if _, err := p.CreateSystemUser(ctx, user); err != nil {
		return fmt.Errorf("bootstrap: create admin user: %w", err)
	}

	bootstrapmeta.LogDefaultAdminCredentials(bootstrapmeta.AdminEmail, bootstrapmeta.DefaultAdminPass)
	log.Println("system bootstrap: default admin user created (SQL)")
	return nil
}

// EnsureBootstrapAdmin creates the default admin user if missing (idempotent).
// Exported for pro layered system bootstrap (e.g. libsql starter provisioned outside OSS driver defaults).
func (p *SystemSQLDriver) EnsureBootstrapAdmin(ctx context.Context) error {
	return p.ensureBootstrapAdmin(ctx)
}

// CreateStarterProjectFn creates the starter project row when it does not exist yet.
// proj has ID, Name, and Description set from bootstrap metadata.
type CreateStarterProjectFn func(ctx context.Context, userID string, proj *models.Project) error

func (p *SystemSQLDriver) ensureBootstrapOrgTeamProject(ctx context.Context) error {
	return p.EnsureBootstrapOrgTeamProjectWithStarterCreate(ctx, func(ctx context.Context, userID string, proj *models.Project) error {
		_, err := p.CreateProject(ctx, userID, proj)
		return err
	})
}

// EnsureBootstrapOrgTeamProjectWithStarterCreate seeds default team/org and links the starter project.
// fn is called only when the starter project row is missing (GetProject returns sql.ErrNoRows).
// Exported for pro builds that replace OSS starter driver creation (e.g. Turso platform provisioning).
func (p *SystemSQLDriver) EnsureBootstrapOrgTeamProjectWithStarterCreate(ctx context.Context, fn CreateStarterProjectFn) error {
	user, err := p.GetSystemUserByEmail(ctx, bootstrapmeta.AdminEmail)
	if err != nil && !bootstrapmeta.IsUserLookupMiss(err) {
		return fmt.Errorf("bootstrap: load admin for org seed: %w", err)
	}
	if user == nil {
		return nil
	}
	if user.DefaultTeamID != "" && user.DefaultOrganizationID != "" {
		return nil
	}

	team := &models.Team{
		Name:        "Default Team",
		Description: "Default team",
		Users: []*models.SystemUser{
			{ID: user.ID, Role: "admin", IsAdmin: true},
		},
	}
	createdTeam, err := p.CreateTeam(ctx, team)
	if err != nil {
		return fmt.Errorf("bootstrap CreateTeam: %w", err)
	}

	org := &models.Organization{
		Name:        "Default Organization",
		Description: "Default organization",
		UserID:      user.ID,
		Teams:       []*models.Team{{ID: createdTeam.ID}},
		Users: []*models.SystemUser{
			{ID: user.ID, Role: "admin", IsAdmin: true},
		},
	}
	createdOrg, err := p.CreateOrganization(ctx, org)
	if err != nil {
		return fmt.Errorf("bootstrap CreateOrganization: %w", err)
	}

	proj := &models.Project{
		ID:          bootstrapmeta.StarterProjectID,
		Name:        bootstrapmeta.StarterProjectName,
		Description: "Starter project",
	}
	if _, err := p.GetProject(ctx, bootstrapmeta.StarterProjectID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("bootstrap GetProject: %w", err)
		}
		if fn == nil {
			return fmt.Errorf("bootstrap: starter project create fn is nil")
		}
		if err := fn(ctx, user.ID, proj); err != nil {
			return fmt.Errorf("bootstrap create starter project: %w", err)
		}
	}

	if err := p.ensureProjectSchemaRow(ctx, bootstrapmeta.StarterProjectID); err != nil {
		return err
	}
	if err := p.ensureProjectSettingsRow(ctx, bootstrapmeta.StarterProjectID); err != nil {
		return err
	}

	if err := p.AssignTeamToOrganization(ctx, createdOrg.ID, user.ID, createdTeam.ID); err != nil && !isSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap AssignTeamToOrganization: %w", err)
	}
	if err := p.AssignProjectToOrganization(ctx, createdOrg.ID, user.ID, bootstrapmeta.StarterProjectID); err != nil && !isSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap AssignProjectToOrganization: %w", err)
	}

	now := time.Now().Format(time.RFC3339)
	uo := &models.UserOrganization{
		UserID:         user.ID,
		OrganizationID: createdOrg.ID,
		Role:           "admin",
		JoinedAt:       now,
	}
	if _, err := p.ORM.NewInsert().Model(uo).Exec(ctx); err != nil && !isSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap user_organizations: %w", err)
	}

	tp := &models.TeamProject{
		TeamID:    createdTeam.ID,
		ProjectID: bootstrapmeta.StarterProjectID,
		LinkedAt:  now,
	}
	if _, err := p.ORM.NewInsert().Model(tp).Exec(ctx); err != nil && !isSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap team_projects: %w", err)
	}

	user.DefaultTeamID = createdTeam.ID
	user.DefaultOrganizationID = createdOrg.ID
	user.DefaultTeam = &models.Team{ID: createdTeam.ID}
	user.DefaultOrganization = &models.Organization{ID: createdOrg.ID}
	user.Projects = []*models.Project{{ID: bootstrapmeta.StarterProjectID}}
	if err := p.UpdateSystemUser(ctx, user, true); err != nil {
		return fmt.Errorf("bootstrap UpdateSystemUser: %w", err)
	}
	log.Println("system bootstrap: default team/org/project linked (SQL)")
	return nil
}

func (p *SystemSQLDriver) ensureProjectSchemaRow(ctx context.Context, projectID string) error {
	exists, err := p.ORM.NewSelect().
		Model((*models.ProjectSchema)(nil)).
		Where("project_id = ?", projectID).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap: check project_schema: %w", err)
	}
	if exists {
		return nil
	}
	row := &models.ProjectSchema{ProjectID: projectID}
	if _, err := p.ORM.NewInsert().Model(row).Exec(ctx); err != nil && !isSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap project_schemas: %w", err)
	}
	return nil
}

// EnsureBootstrapProjectSettingsRow ensures a project_settings row exists (idempotent).
// Exported for pro layered bootstrap that skips open-core ensureStarterProjectDriver.
func (p *SystemSQLDriver) EnsureBootstrapProjectSettingsRow(ctx context.Context, projectID string) error {
	return p.ensureProjectSettingsRow(ctx, projectID)
}

func (p *SystemSQLDriver) ensureProjectSettingsRow(ctx context.Context, projectID string) error {
	exists, err := p.ORM.NewSelect().
		Model((*models.ProjectSettings)(nil)).
		Where("project_id = ?", projectID).
		Exists(ctx)
	if err != nil {
		return fmt.Errorf("bootstrap: check project_settings: %w", err)
	}
	if exists {
		return nil
	}
	row := &models.ProjectSettings{
		ProjectID: projectID,
		Locals:    []string{"en"},
	}
	if _, err := p.ORM.NewInsert().Model(row).Exec(ctx); err != nil && !isSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap project_settings: %w", err)
	}
	return nil
}

// ensureStarterProjectDriver backfills driver_credentials for the starter project when config is present
// (e.g. user already had default org/team and ensureBootstrapOrgTeamProject returned early).
func (p *SystemSQLDriver) ensureStarterProjectDriver(ctx context.Context) error {
	if err := p.ensureProjectDriverCredentials(ctx, bootstrapmeta.StarterProjectID); err != nil {
		return fmt.Errorf("bootstrap starter project driver: %w", err)
	}
	if err := p.ensureProjectSettingsRow(ctx, bootstrapmeta.StarterProjectID); err != nil {
		return fmt.Errorf("bootstrap starter project settings: %w", err)
	}
	return nil
}

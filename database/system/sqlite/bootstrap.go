package sqlite

import (
	"github.com/apito-io/engine/database/system/sqlcommon"
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

func (d *Driver) ensureBootstrapAdmin(ctx context.Context) error {
	u, err := d.GetSystemUserByEmail(ctx, bootstrapmeta.AdminEmail)
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

	if _, err := d.CreateSystemUser(ctx, user); err != nil {
		return fmt.Errorf("bootstrap: create admin user: %w", err)
	}

	bootstrapmeta.LogDefaultAdminCredentials(bootstrapmeta.AdminEmail, bootstrapmeta.DefaultAdminPass)
	log.Println("system bootstrap: default admin user created (SQL)")
	return nil
}

// EnsureBootstrapAdmin creates the default admin user if missing (idempotent).
// Exported for pro layered system bootstrap (e.g. libsql starter provisioned outside OSS driver defaults).
func (d *Driver) EnsureBootstrapAdmin(ctx context.Context) error {
	return d.ensureBootstrapAdmin(ctx)
}

func (d *Driver) ensureBootstrapOrgTeamProject(ctx context.Context) error {
	return d.EnsureBootstrapOrgTeamProjectWithStarterCreate(ctx, func(ctx context.Context, userID string, proj *models.Project) error {
		_, err := d.CreateProject(ctx, userID, proj)
		return err
	})
}

// EnsureBootstrapOrgTeamProjectWithStarterCreate seeds default team/org and links the starter project.
// fn is called only when the starter project row is missing (GetProject returns sql.ErrNoRows).
// Exported for pro builds that replace OSS starter driver creation (e.g. Turso platform provisioning).
func (d *Driver) EnsureBootstrapOrgTeamProjectWithStarterCreate(ctx context.Context, fn bootstrapmeta.CreateStarterProjectFn) error {
	user, err := d.GetSystemUserByEmail(ctx, bootstrapmeta.AdminEmail)
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
	createdTeam, err := d.CreateTeam(ctx, team)
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
	createdOrg, err := d.CreateOrganization(ctx, org)
	if err != nil {
		return fmt.Errorf("bootstrap CreateOrganization: %w", err)
	}

	proj := &models.Project{
		ID:          bootstrapmeta.StarterProjectID,
		Name:        bootstrapmeta.StarterProjectName,
		Description: "Starter project",
	}
	if _, err := d.GetProject(ctx, bootstrapmeta.StarterProjectID); err != nil {
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

	if err := d.ensureProjectSchemaRow(ctx, bootstrapmeta.StarterProjectID); err != nil {
		return err
	}
	if err := d.ensureProjectSettingsRow(ctx, bootstrapmeta.StarterProjectID); err != nil {
		return err
	}

	if err := d.AssignTeamToOrganization(ctx, createdOrg.ID, user.ID, createdTeam.ID); err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap AssignTeamToOrganization: %w", err)
	}
	if err := d.AssignProjectToOrganization(ctx, createdOrg.ID, user.ID, bootstrapmeta.StarterProjectID); err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap AssignProjectToOrganization: %w", err)
	}

	now := time.Now().Format(time.RFC3339)
	uo := &models.UserOrganization{
		UserID:         user.ID,
		OrganizationID: createdOrg.ID,
		Role:           "admin",
		JoinedAt:       now,
	}
	if _, err := d.ORM.NewInsert().Model(uo).Exec(ctx); err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap user_organizations: %w", err)
	}

	tp := &models.TeamProject{
		TeamID:    createdTeam.ID,
		ProjectID: bootstrapmeta.StarterProjectID,
		LinkedAt:  now,
	}
	if _, err := d.ORM.NewInsert().Model(tp).Exec(ctx); err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap team_projects: %w", err)
	}

	user.DefaultTeamID = createdTeam.ID
	user.DefaultOrganizationID = createdOrg.ID
	user.DefaultTeam = &models.Team{ID: createdTeam.ID}
	user.DefaultOrganization = &models.Organization{ID: createdOrg.ID}
	user.Projects = []*models.Project{{ID: bootstrapmeta.StarterProjectID}}
	if err := d.UpdateSystemUser(ctx, user, true); err != nil {
		return fmt.Errorf("bootstrap UpdateSystemUser: %w", err)
	}
	log.Println("system bootstrap: default team/org/project linked (SQL)")
	return nil
}

func (d *Driver) repairProjectSchemaIntegrity(ctx context.Context) error {
	if _, err := d.ORM.NewDelete().
		Model((*models.ModelType)(nil)).
		Where("project_id = '' OR project_id IS NULL").
		Exec(ctx); err != nil {
		return fmt.Errorf("repair model_types orphans: %w", err)
	}
	if _, err := d.ORM.NewDelete().
		Model((*models.ApitoFunction)(nil)).
		Where("project_id = '' OR project_id IS NULL").
		Exec(ctx); err != nil {
		return fmt.Errorf("repair apito_functions orphans: %w", err)
	}

	var projectIDs []string
	if err := d.ORM.NewSelect().
		Model((*models.Project)(nil)).
		Column("id").
		Scan(ctx, &projectIDs); err != nil {
		return fmt.Errorf("repair project_schemas list projects: %w", err)
	}
	for _, projectID := range projectIDs {
		if projectID == "" {
			continue
		}
		if err := d.ensureProjectSchemaRow(ctx, projectID); err != nil {
			return err
		}
	}
	return nil
}

func (d *Driver) ensureProjectSchemaRow(ctx context.Context, projectID string) error {
	exists, err := d.ORM.NewSelect().
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
	if _, err := d.ORM.NewInsert().Model(row).Exec(ctx); err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap project_schemas: %w", err)
	}
	return nil
}

// EnsureBootstrapProjectSettingsRow ensures a project_settings row exists (idempotent).
// Exported for pro layered bootstrap that skips open-core ensureStarterProjectDriver.
func (d *Driver) EnsureBootstrapProjectSettingsRow(ctx context.Context, projectID string) error {
	return d.ensureProjectSettingsRow(ctx, projectID)
}

func (d *Driver) ensureProjectSettingsRow(ctx context.Context, projectID string) error {
	exists, err := d.ORM.NewSelect().
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
	if _, err := d.ORM.NewInsert().Model(row).Exec(ctx); err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
		return fmt.Errorf("bootstrap project_settings: %w", err)
	}
	return nil
}

// ensureStarterProjectDriver backfills driver_credentials for the starter project when config is present
// (e.g. user already had default org/team and ensureBootstrapOrgTeamProject returned early).
func (d *Driver) ensureStarterProjectDriver(ctx context.Context) error {
	if err := d.ensureProjectDriverCredentials(ctx, bootstrapmeta.StarterProjectID); err != nil {
		return fmt.Errorf("bootstrap starter project driver: %w", err)
	}
	if err := d.ensureProjectSettingsRow(ctx, bootstrapmeta.StarterProjectID); err != nil {
		return fmt.Errorf("bootstrap starter project settings: %w", err)
	}
	return nil
}

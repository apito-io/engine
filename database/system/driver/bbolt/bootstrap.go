package bbolt

import (
	"context"
	"fmt"
	"log"

	"github.com/apito-io/engine/database/system/bootstrapmeta"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/google/uuid"
)

// EnsureSystemBootstrap creates default admin, then default team/org/project via junction collections.
func (d *ProBBoltSystemDriver) EnsureSystemBootstrap(ctx context.Context) error {
	if err := d.ensureBootstrapAdmin(ctx); err != nil {
		return err
	}
	return d.ensureBootstrapOrgTeamProject(ctx)
}

func (d *ProBBoltSystemDriver) ensureBootstrapAdmin(ctx context.Context) error {
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

	id := uuid.New().String()
	now := utility.GetCurrentTime()
	user := &models.SystemUser{
		XKey:                      id,
		ID:                        id,
		Email:                     bootstrapmeta.AdminEmail,
		Username:                  uuid.New().String(),
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
	log.Println("system bootstrap: default admin user created (bBolt)")
	return nil
}

func (d *ProBBoltSystemDriver) ensureBootstrapOrgTeamProject(ctx context.Context) error {
	user, err := d.GetSystemUserByEmail(ctx, bootstrapmeta.AdminEmail)
	if err != nil && !bootstrapmeta.IsUserLookupMiss(err) {
		return fmt.Errorf("bootstrap: load admin for org seed: %w", err)
	}
	if user == nil {
		return nil
	}
	if user.DefaultTeam != nil && user.DefaultOrganization != nil {
		return nil
	}

	team := &models.Team{Name: "Default Team", Description: "Default team"}
	createdTeam, err := d.CreateTeam(ctx, team)
	if err != nil {
		return fmt.Errorf("bootstrap CreateTeam: %w", err)
	}

	org := &models.Organization{Name: "Default Organization", Description: "Default organization"}
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
		if _, err := d.CreateProject(ctx, user.ID, proj); err != nil {
			return fmt.Errorf("bootstrap CreateProject: %w", err)
		}
	}

	ts := utility.GetCurrentTime()
	_ = d.SaveRawData(ctx, "user_teams", map[string]interface{}{
		"user_id": user.ID, "team_id": createdTeam.ID,
	})
	_ = d.SaveRawData(ctx, "user_projects", map[string]interface{}{
		"user_id": user.ID, "project_id": bootstrapmeta.StarterProjectID,
		"role": "owner", "permissions": []string{"read", "write", "admin"},
	})
	_ = d.SaveRawData(ctx, "user_organizations", map[string]interface{}{
		"user_id": user.ID, "organization_id": createdOrg.ID, "role": "admin",
	})
	_ = d.SaveRawData(ctx, "organization_teams", map[string]interface{}{
		"organization_id": createdOrg.ID, "team_id": createdTeam.ID,
		"assigned_by": user.ID, "assigned_at": ts,
	})
	_ = d.SaveRawData(ctx, "organization_projects", map[string]interface{}{
		"organization_id": createdOrg.ID, "project_id": bootstrapmeta.StarterProjectID,
		"assigned_by": user.ID, "assigned_at": ts,
	})
	_ = d.SaveRawData(ctx, "team_projects", map[string]interface{}{
		"team_id": createdTeam.ID, "project_id": bootstrapmeta.StarterProjectID,
	})

	user.DefaultTeam = &models.Team{ID: createdTeam.ID}
	user.DefaultOrganization = &models.Organization{ID: createdOrg.ID}
	user.Projects = []*models.Project{{ID: bootstrapmeta.StarterProjectID}}
	if err := d.UpdateSystemUser(ctx, user, true); err != nil {
		return fmt.Errorf("bootstrap UpdateSystemUser: %w", err)
	}
	log.Println("system bootstrap: default team/org/project linked (bBolt)")
	return nil
}

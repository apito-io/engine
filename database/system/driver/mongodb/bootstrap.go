package mongodb

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/apito-io/engine/database/system/bootstrapmeta"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func mongoIsDup(err error) bool {
	var ce mongo.CommandError
	if errors.As(err, &ce) {
		return ce.HasErrorCode(11000)
	}
	return false
}

// EnsureSystemBootstrap creates default admin, then default team/org/project and junction rows when needed.
func (m *SystemMongoDriver) EnsureSystemBootstrap(ctx context.Context) error {
	if err := m.ensureBootstrapAdmin(ctx); err != nil {
		return err
	}
	return m.ensureBootstrapOrgTeamProject(ctx)
}

func (m *SystemMongoDriver) ensureBootstrapAdmin(ctx context.Context) error {
	u, err := m.GetSystemUserByEmail(ctx, bootstrapmeta.AdminEmail)
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

	if _, err := m.CreateSystemUser(ctx, user); err != nil {
		return fmt.Errorf("bootstrap: create admin user: %w", err)
	}

	bootstrapmeta.LogDefaultAdminCredentials(bootstrapmeta.AdminEmail, bootstrapmeta.DefaultAdminPass)
	log.Println("system bootstrap: default admin user created (Mongo)")
	return nil
}

func (m *SystemMongoDriver) ensureBootstrapOrgTeamProject(ctx context.Context) error {
	user, err := m.GetSystemUserByEmail(ctx, bootstrapmeta.AdminEmail)
	if err != nil && !bootstrapmeta.IsUserLookupMiss(err) {
		return fmt.Errorf("bootstrap: load admin for org seed: %w", err)
	}
	if user == nil {
		return nil
	}
	if user.DefaultTeam != nil && user.DefaultOrganization != nil {
		return nil
	}

	team := &models.Team{
		Name:        "Default Team",
		Description: "Default team",
		Users: []*models.SystemUser{
			{ID: user.ID, Role: "admin", IsAdmin: true},
		},
	}
	createdTeam, err := m.CreateTeam(ctx, team)
	if err != nil {
		return fmt.Errorf("bootstrap CreateTeam: %w", err)
	}

	org := &models.Organization{
		Name:        "Default Organization",
		Description: "Default organization",
		Teams:       []*models.Team{{ID: createdTeam.ID}},
		Users: []*models.SystemUser{
			{ID: user.ID, Role: "admin", IsAdmin: true},
		},
	}
	createdOrg, err := m.CreateOrganization(ctx, org)
	if err != nil {
		return fmt.Errorf("bootstrap CreateOrganization: %w", err)
	}

	proj := &models.Project{
		ID:          bootstrapmeta.StarterProjectID,
		Name:        bootstrapmeta.StarterProjectName,
		Description: "Starter project",
	}
	if _, err := m.GetProject(ctx, bootstrapmeta.StarterProjectID); err != nil {
		if _, err := m.CreateProject(ctx, user.ID, proj); err != nil {
			return fmt.Errorf("bootstrap CreateProject: %w", err)
		}
	}

	if err := m.AssignTeamToOrganization(ctx, createdOrg.ID, user.ID, createdTeam.ID); err != nil {
		return fmt.Errorf("bootstrap AssignTeamToOrganization: %w", err)
	}
	if err := m.AssignProjectToOrganization(ctx, createdOrg.ID, user.ID, bootstrapmeta.StarterProjectID); err != nil {
		return fmt.Errorf("bootstrap AssignProjectToOrganization: %w", err)
	}

	now := time.Now()
	uoColl := m.Database.Collection("user_organizations")
	_, err = uoColl.InsertOne(ctx, bson.M{
		"user_id":         user.ID,
		"organization_id": createdOrg.ID,
		"role":            "admin",
		"joined_at":       now,
	})
	if err != nil && !mongoIsDup(err) {
		return fmt.Errorf("bootstrap user_organizations: %w", err)
	}

	tpColl := m.Database.Collection("team_projects")
	_, err = tpColl.InsertOne(ctx, bson.M{
		"team_id":    createdTeam.ID,
		"project_id": bootstrapmeta.StarterProjectID,
		"linked_at":  now,
	})
	if err != nil && !mongoIsDup(err) {
		return fmt.Errorf("bootstrap team_projects: %w", err)
	}

	user.DefaultTeam = &models.Team{ID: createdTeam.ID}
	user.DefaultOrganization = &models.Organization{ID: createdOrg.ID}
	user.Projects = []*models.Project{{ID: bootstrapmeta.StarterProjectID}}
	if err := m.UpdateSystemUser(ctx, user, true); err != nil {
		return fmt.Errorf("bootstrap UpdateSystemUser: %w", err)
	}
	log.Println("system bootstrap: default team/org/project linked (Mongo)")
	return nil
}

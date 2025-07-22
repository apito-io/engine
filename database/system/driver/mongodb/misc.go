package mongodb

import (
	"context"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// GetProjects retrieves multiple projects by their IDs using MongoDB
func (m *SystemMongoDriver) GetProjects(ctx context.Context, keys []string) ([]*models.Project, error) {
	collection := m.Database.Collection("projects")

	filter := bson.M{"_id": bson.M{"$in": keys}}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var projects []*models.Project
	for cursor.Next(ctx) {
		var project models.Project
		if err := cursor.Decode(&project); err != nil {
			return nil, err
		}
		projects = append(projects, &project)
	}

	return projects, nil
}

// AddATeamMemberToProject adds a team member to a project using MongoDB
func (m *SystemMongoDriver) AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error {
	// Create user-project relation with role
	userProjectsCollection := m.Database.Collection("user_projects")
	_, err := userProjectsCollection.InsertOne(ctx, map[string]interface{}{
		"user_id":     req.UserID,
		"project_id":  req.ProjectID,
		"role":        req.Role,
		"permissions": req.Permissions,
	})

	if err != nil {
		return err
	}

	// Also add to project_teams collection
	projectTeamsCollection := m.Database.Collection("project_teams")
	_, err = projectTeamsCollection.InsertOne(ctx, map[string]interface{}{
		"user_id":    req.UserID,
		"project_id": req.ProjectID,
	})

	return err
}

// GetSystemUsers retrieves multiple system users by their IDs using MongoDB
func (m *SystemMongoDriver) GetSystemUsers(ctx context.Context, keys []string) ([]*models.SystemUser, error) {
	collection := m.Database.Collection("users")

	filter := bson.M{"_id": bson.M{"$in": keys}}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []*models.SystemUser
	for cursor.Next(ctx) {
		var user models.SystemUser
		if err := cursor.Decode(&user); err != nil {
			return nil, err
		}
		users = append(users, &user)
	}

	return users, nil
}

// FindUserOrganizations retrieves all organizations for a given user using MongoDB
func (m *SystemMongoDriver) FindUserOrganizations(ctx context.Context, userId string) ([]*models.Organization, error) {
	userOrganizationsCollection := m.Database.Collection("user_organizations")

	// Find organization IDs for the user
	cursor, err := userOrganizationsCollection.Find(ctx, bson.M{"user_id": userId})
	if err != nil {
		return nil, err
	}

	var userOrgs []map[string]interface{}
	if err = cursor.All(ctx, &userOrgs); err != nil {
		return nil, err
	}
	cursor.Close(ctx)

	var orgIds []string
	for _, uo := range userOrgs {
		if orgId, ok := uo["organization_id"].(string); ok {
			orgIds = append(orgIds, orgId)
		}
	}

	// Get organizations
	organizationsCollection := m.Database.Collection("organizations")
	orgCursor, err := organizationsCollection.Find(ctx, bson.M{"_id": bson.M{"$in": orgIds}})
	if err != nil {
		return nil, err
	}
	defer orgCursor.Close(ctx)

	var organizations []*models.Organization
	for orgCursor.Next(ctx) {
		var org models.Organization
		if err := orgCursor.Decode(&org); err != nil {
			return nil, err
		}
		organizations = append(organizations, &org)
	}

	return organizations, nil
}

// CreateOrganization creates a new organization using MongoDB
func (m *SystemMongoDriver) CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error) {
	if org.ID == "" {
		org.ID = uuid.New().String()
	}

	collection := m.Database.Collection("organizations")
	_, err := collection.InsertOne(ctx, org)

	return org, err
}

// AssignTeamToOrganization assigns a team to an organization using MongoDB
func (m *SystemMongoDriver) AssignTeamToOrganization(ctx context.Context, orgId, userId, teamId string) error {
	collection := m.Database.Collection("organization_teams")

	_, err := collection.InsertOne(ctx, map[string]interface{}{
		"organization_id": orgId,
		"team_id":         teamId,
		"assigned_by":     userId,
		"assigned_at":     time.Now(),
	})

	return err
}

// RemoveATeamFromOrganization removes a team from an organization using MongoDB
func (m *SystemMongoDriver) RemoveATeamFromOrganization(ctx context.Context, orgId, userId, teamId string) error {
	collection := m.Database.Collection("organization_teams")

	_, err := collection.DeleteMany(ctx, bson.M{
		"organization_id": orgId,
		"team_id":         teamId,
	})

	return err
}

// AssignProjectToOrganization assigns a project to an organization using MongoDB
func (m *SystemMongoDriver) AssignProjectToOrganization(ctx context.Context, orgId, userId, projectId string) error {
	collection := m.Database.Collection("organization_projects")

	_, err := collection.InsertOne(ctx, map[string]interface{}{
		"organization_id": orgId,
		"project_id":      projectId,
		"assigned_by":     userId,
		"assigned_at":     time.Now(),
	})

	return err
}

// RemoveProjectFromOrganization removes a project from an organization using MongoDB
func (m *SystemMongoDriver) RemoveProjectFromOrganization(ctx context.Context, orgId, userId, projectId string) error {
	collection := m.Database.Collection("organization_projects")

	_, err := collection.DeleteMany(ctx, bson.M{
		"organization_id": orgId,
		"project_id":      projectId,
	})

	return err
}

// GetProjectTeams retrieves team information for a project using MongoDB
func (m *SystemMongoDriver) GetProjectTeams(ctx context.Context, projectId string) (*models.Team, error) {
	teamProjectsCollection := m.Database.Collection("team_projects")

	// Find team ID for the project
	var teamProject map[string]interface{}
	err := teamProjectsCollection.FindOne(ctx, bson.M{"project_id": projectId}).Decode(&teamProject)
	if err != nil {
		return nil, err
	}

	teamId, ok := teamProject["team_id"].(string)
	if !ok {
		return nil, fmt.Errorf("team ID not found")
	}

	// Get the team
	teamsCollection := m.Database.Collection("teams")
	var team models.Team
	err = teamsCollection.FindOne(ctx, bson.M{"_id": teamId}).Decode(&team)

	return &team, err
}

// FindUserTeams retrieves all teams for a given user using MongoDB
func (m *SystemMongoDriver) FindUserTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	userTeamsCollection := m.Database.Collection("user_teams")

	// Find team IDs for the user
	cursor, err := userTeamsCollection.Find(ctx, bson.M{"user_id": userId})
	if err != nil {
		return nil, err
	}

	var userTeams []map[string]interface{}
	if err = cursor.All(ctx, &userTeams); err != nil {
		return nil, err
	}
	cursor.Close(ctx)

	var teamIds []string
	for _, ut := range userTeams {
		if teamId, ok := ut["team_id"].(string); ok {
			teamIds = append(teamIds, teamId)
		}
	}

	// Get teams
	teamsCollection := m.Database.Collection("teams")
	teamCursor, err := teamsCollection.Find(ctx, bson.M{"_id": bson.M{"$in": teamIds}})
	if err != nil {
		return nil, err
	}
	defer teamCursor.Close(ctx)

	var teams []*models.Team
	for teamCursor.Next(ctx) {
		var team models.Team
		if err := teamCursor.Decode(&team); err != nil {
			return nil, err
		}
		teams = append(teams, &team)
	}

	return teams, nil
}

// CreateTeam creates a new team using MongoDB
func (m *SystemMongoDriver) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	if team.ID == "" {
		team.ID = uuid.New().String()
	}

	teamsCollection := m.Database.Collection("teams")
	_, err := teamsCollection.InsertOne(ctx, team)
	if err != nil {
		return nil, err
	}

	// Create user-team relations for each user
	userTeamsCollection := m.Database.Collection("user_teams")
	for _, user := range team.Users {
		_, err = userTeamsCollection.InsertOne(ctx, map[string]interface{}{
			"user_id": user.ID,
			"team_id": team.ID,
		})

		if err != nil {
			return nil, err
		}
	}

	return team, nil
}

// FindUserProjects retrieves all projects for a given user using MongoDB
func (m *SystemMongoDriver) FindUserProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	userProjectsCollection := m.Database.Collection("user_projects")

	// Find project IDs for the user
	cursor, err := userProjectsCollection.Find(ctx, bson.M{"user_id": userId})
	if err != nil {
		return nil, err
	}

	var userProjects []map[string]interface{}
	if err = cursor.All(ctx, &userProjects); err != nil {
		return nil, err
	}
	cursor.Close(ctx)

	var projectIds []string
	for _, up := range userProjects {
		if projectId, ok := up["project_id"].(string); ok {
			projectIds = append(projectIds, projectId)
		}
	}

	// Get projects
	projectsCollection := m.Database.Collection("projects")
	projectCursor, err := projectsCollection.Find(ctx, bson.M{"_id": bson.M{"$in": projectIds}})
	if err != nil {
		return nil, err
	}
	defer projectCursor.Close(ctx)

	var projects []*models.Project
	for projectCursor.Next(ctx) {
		var project models.Project
		if err := projectCursor.Decode(&project); err != nil {
			return nil, err
		}
		projects = append(projects, &project)
	}

	return projects, nil
}

// CheckProjectWithRoles checks if a user belongs to a project and returns roles/permissions using MongoDB
func (m *SystemMongoDriver) CheckProjectWithRoles(ctx context.Context, userId, projectId string) (*models.ProjectWithRoles, error) {
	if projectId == "" {
		return nil, fmt.Errorf("project id is empty")
	}

	// Get the project
	project, err := m.GetProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	// Get the user
	user, err := m.GetSystemUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	// Get user-project relation with role and permissions
	userProjectsCollection := m.Database.Collection("user_projects")
	var relation map[string]interface{}
	err = userProjectsCollection.FindOne(ctx, bson.M{
		"user_id":    userId,
		"project_id": projectId,
	}).Decode(&relation)
	if err != nil {
		return nil, err
	}

	role, _ := relation["role"].(string)
	permissions, _ := relation["permissions"].([]interface{})

	var permList []string
	for _, perm := range permissions {
		if p, ok := perm.(string); ok {
			permList = append(permList, p)
		}
	}

	return &models.ProjectWithRoles{
		User:        user,
		Project:     project,
		Role:        role,
		Permissions: permList,
	}, nil
}

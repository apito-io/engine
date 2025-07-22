package couchbase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/couchbase/gocb/v2"
	"github.com/google/uuid"
)

// GetProjects retrieves multiple projects by their IDs using Couchbase
func (c *CouchbaseDriver) GetProjects(ctx context.Context, keys []string) ([]*models.Project, error) {
	var projects []*models.Project

	for _, key := range keys {
		project, err := c.GetProject(ctx, key)
		if err != nil {
			continue // Skip missing projects
		}
		projects = append(projects, project)
	}

	return projects, nil
}

// AddATeamMemberToProject adds a team member to a project using Couchbase
func (c *CouchbaseDriver) AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error {
	// Get the user to add
	user, err := c.GetSystemUser(ctx, req.UserID)
	if err != nil {
		return err
	}

	userDataJson, err := json.Marshal(user)
	if err != nil {
		return err
	}

	// Store team member in project
	projectTeamDoc := map[string]interface{}{
		"project_id": req.ProjectID,
		"user_id":    req.UserID,
		"user_data":  string(userDataJson),
		"doc_type":   "project_team",
	}

	_, err = c.Collection.Upsert(fmt.Sprintf("project_team::%s::%s", req.ProjectID, req.UserID), projectTeamDoc, nil)
	if err != nil {
		return err
	}

	// Store user-project relation with role
	project, err := c.GetProject(ctx, req.ProjectID)
	if err != nil {
		return err
	}

	projectDataJson, _ := json.Marshal(project)
	permissionsJson, _ := json.Marshal(req.Permissions)

	userProjectDoc := map[string]interface{}{
		"user_id":      req.UserID,
		"project_id":   req.ProjectID,
		"project_data": string(projectDataJson),
		"role":         req.Role,
		"permissions":  string(permissionsJson),
		"doc_type":     "user_project",
	}

	_, err = c.Collection.Upsert(fmt.Sprintf("user_project::%s::%s", req.UserID, req.ProjectID), userProjectDoc, nil)
	return err
}

// GetSystemUsers retrieves multiple system users by their IDs using Couchbase
func (c *CouchbaseDriver) GetSystemUsers(ctx context.Context, keys []string) ([]*models.SystemUser, error) {
	var users []*models.SystemUser

	for _, key := range keys {
		user, err := c.GetSystemUser(ctx, key)
		if err != nil {
			continue // Skip missing users
		}
		users = append(users, user)
	}

	return users, nil
}

// FindUserOrganizations retrieves all organizations for a given user using Couchbase
func (c *CouchbaseDriver) FindUserOrganizations(ctx context.Context, userId string) ([]*models.Organization, error) {
	query := fmt.Sprintf("SELECT organization_data FROM `%s` WHERE doc_type = \"user_organization\" AND user_id = $1", c.Bucket.Name())

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{userId},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var organizations []*models.Organization
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if orgDataStr, ok := row["organization_data"].(string); ok {
			var org models.Organization
			if err := json.Unmarshal([]byte(orgDataStr), &org); err == nil {
				organizations = append(organizations, &org)
			}
		}
	}

	return organizations, nil
}

// CreateOrganization creates a new organization using Couchbase
func (c *CouchbaseDriver) CreateOrganization(ctx context.Context, org *models.Organization) (*models.Organization, error) {
	if org.ID == "" {
		org.ID = uuid.New().String()
	}

	orgDataJson, err := json.Marshal(org)
	if err != nil {
		return nil, err
	}

	doc := map[string]interface{}{
		"id":                org.ID,
		"name":              org.Name,
		"organization_data": string(orgDataJson),
		"doc_type":          "organization",
	}

	_, err = c.Collection.Upsert("organization::"+org.ID, doc, nil)
	return org, err
}

// AssignTeamToOrganization assigns a team to an organization using Couchbase
func (c *CouchbaseDriver) AssignTeamToOrganization(ctx context.Context, orgId, userId, teamId string) error {
	doc := map[string]interface{}{
		"organization_id": orgId,
		"team_id":         teamId,
		"assigned_by":     userId,
		"assigned_at":     time.Now().Format(time.RFC3339),
		"doc_type":        "organization_team",
	}

	_, err := c.Collection.Upsert(fmt.Sprintf("organization_team::%s::%s", orgId, teamId), doc, nil)
	return err
}

// RemoveATeamFromOrganization removes a team from an organization using Couchbase
func (c *CouchbaseDriver) RemoveATeamFromOrganization(ctx context.Context, orgId, userId, teamId string) error {
	_, err := c.Collection.Remove(fmt.Sprintf("organization_team::%s::%s", orgId, teamId), nil)
	return err
}

// AssignProjectToOrganization assigns a project to an organization using Couchbase
func (c *CouchbaseDriver) AssignProjectToOrganization(ctx context.Context, orgId, userId, projectId string) error {
	doc := map[string]interface{}{
		"organization_id": orgId,
		"project_id":      projectId,
		"assigned_by":     userId,
		"assigned_at":     time.Now().Format(time.RFC3339),
		"doc_type":        "organization_project",
	}

	_, err := c.Collection.Upsert(fmt.Sprintf("organization_project::%s::%s", orgId, projectId), doc, nil)
	return err
}

// RemoveProjectFromOrganization removes a project from an organization using Couchbase
func (c *CouchbaseDriver) RemoveProjectFromOrganization(ctx context.Context, orgId, userId, projectId string) error {
	_, err := c.Collection.Remove(fmt.Sprintf("organization_project::%s::%s", orgId, projectId), nil)
	return err
}

// GetProjectTeams retrieves team information for a project using Couchbase
func (c *CouchbaseDriver) GetProjectTeams(ctx context.Context, projectId string) (*models.Team, error) {
	query := fmt.Sprintf("SELECT team_data FROM `%s` WHERE doc_type = \"team_project\" AND project_id = $1 LIMIT 1", c.Bucket.Name())

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{projectId},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	if !results.Next() {
		return nil, fmt.Errorf("team not found for project")
	}

	var row map[string]interface{}
	if err := results.Row(&row); err != nil {
		return nil, err
	}

	if teamDataStr, ok := row["team_data"].(string); ok {
		var team models.Team
		if err := json.Unmarshal([]byte(teamDataStr), &team); err != nil {
			return nil, err
		}
		return &team, nil
	}

	return nil, fmt.Errorf("team data not found")
}

// FindUserTeams retrieves all teams for a given user using Couchbase
func (c *CouchbaseDriver) FindUserTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	query := fmt.Sprintf("SELECT team_data FROM `%s` WHERE doc_type = \"user_team\" AND user_id = $1", c.Bucket.Name())

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{userId},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var teams []*models.Team
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if teamDataStr, ok := row["team_data"].(string); ok {
			var team models.Team
			if err := json.Unmarshal([]byte(teamDataStr), &team); err == nil {
				teams = append(teams, &team)
			}
		}
	}

	return teams, nil
}

// CreateTeam creates a new team using Couchbase
func (c *CouchbaseDriver) CreateTeam(ctx context.Context, team *models.Team) (*models.Team, error) {
	if team.ID == "" {
		team.ID = uuid.New().String()
	}

	teamDataJson, err := json.Marshal(team)
	if err != nil {
		return nil, err
	}

	// Store the team
	teamDoc := map[string]interface{}{
		"id":        team.ID,
		"name":      team.Name,
		"team_data": string(teamDataJson),
		"doc_type":  "team",
	}

	_, err = c.Collection.Upsert("team::"+team.ID, teamDoc, nil)
	if err != nil {
		return nil, err
	}

	// Store user-team relations for each user
	for _, user := range team.Users {
		userTeamDoc := map[string]interface{}{
			"user_id":   user.ID,
			"team_id":   team.ID,
			"team_data": string(teamDataJson),
			"doc_type":  "user_team",
		}

		_, err = c.Collection.Upsert(fmt.Sprintf("user_team::%s::%s", user.ID, team.ID), userTeamDoc, nil)
		if err != nil {
			return nil, err
		}
	}

	return team, nil
}

// FindUserProjects retrieves all projects for a given user using Couchbase
func (c *CouchbaseDriver) FindUserProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	query := fmt.Sprintf("SELECT project_data FROM `%s` WHERE doc_type = \"user_project\" AND user_id = $1", c.Bucket.Name())

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{userId},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var projects []*models.Project
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if projectDataStr, ok := row["project_data"].(string); ok {
			var project models.Project
			if err := json.Unmarshal([]byte(projectDataStr), &project); err == nil {
				projects = append(projects, &project)
			}
		}
	}

	return projects, nil
}

// CheckProjectWithRoles checks if a user belongs to a project and returns roles/permissions using Couchbase
func (c *CouchbaseDriver) CheckProjectWithRoles(ctx context.Context, userId, projectId string) (*models.ProjectWithRoles, error) {
	if projectId == "" {
		return nil, fmt.Errorf("project id is empty")
	}

	// Get the project
	project, err := c.GetProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	// Get the user
	user, err := c.GetSystemUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	// Get user-project relation with role and permissions
	query := fmt.Sprintf("SELECT role, permissions FROM `%s` WHERE doc_type = \"user_project\" AND user_id = $1 AND project_id = $2 LIMIT 1", c.Bucket.Name())

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{userId, projectId},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	if !results.Next() {
		return nil, fmt.Errorf("user is not associated with this project")
	}

	var row map[string]interface{}
	if err := results.Row(&row); err != nil {
		return nil, err
	}

	role, _ := row["role"].(string)
	permissionsStr, _ := row["permissions"].(string)

	var permissions []string
	if permissionsStr != "" {
		json.Unmarshal([]byte(permissionsStr), &permissions)
	}

	return &models.ProjectWithRoles{
		User:        user,
		Project:     project,
		Role:        role,
		Permissions: permissions,
	}, nil
}

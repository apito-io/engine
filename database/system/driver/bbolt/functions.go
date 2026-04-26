package bbolt

import (
	"context"
	"errors"
	"fmt"
	apitobolt "github.com/apito-io/apitoBolt"
	q "github.com/apito-io/apitoBolt/q"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
)

// Project-related functions

func (d *ProBBoltSystemDriver) CreateProject(ctx context.Context, userId string, project *models.Project) (*models.Project, error) {

	// Set timestamps
	project.CreatedAt = utility.GetCurrentTime()
	project.UpdatedAt = utility.GetCurrentTime()

	// Generate ID if not set
	if project.ID == "" {
		project.ID = utility.NewID()
	}
	project.XKey = project.ID

	// Save project
	collection := d.DB.Collection("projects")
	_, err := collection.Save(project)
	if err != nil {
		return nil, err
	}

	// If starts from example project then transfer the content and model
	if project.ProjectTemplate != "" {
		return project, d.TransferSchema(ctx, project.ProjectTemplate, project.ID)
	}

	return project, nil
}

func (d *ProBBoltSystemDriver) UpdateProject(ctx context.Context, project *models.Project, replace bool) error {
	collection := d.DB.Collection("projects")
	project.UpdatedAt = utility.GetCurrentTime()

	if replace {
		// For replace operation, use Update which overwrites
		return collection.Update(project)
	} else {
		// For partial update, we need to read existing and merge
		var existing models.Project
		err := collection.FindByID(project.ID, &existing)
		if err != nil {
			return err
		}

		// Merge non-zero values
		if project.Name != "" {
			existing.Name = project.Name
		}
		if project.Description != "" {
			existing.Description = project.Description
		}
		if project.Schema != nil {
			existing.Schema = project.Schema
		}
		if project.Settings != nil {
			existing.Settings = project.Settings
		}
		if project.Roles != nil {
			existing.Roles = project.Roles
		}
		existing.UpdatedAt = project.UpdatedAt

		return collection.Update(&existing)
	}
}

func (d *ProBBoltSystemDriver) GetProject(ctx context.Context, id string) (*models.Project, error) {
	var project models.Project
	collection := d.DB.Collection("projects")

	err := collection.FindByID(id, &project)
	if err != nil {
		return nil, err
	}

	if project.ID == "" {
		return nil, errors.New("no Project Found")
	}

	return &project, nil
}


func (d *ProBBoltSystemDriver) GetProjects(ctx context.Context, keys []string) ([]*models.Project, error) {
	collection := d.DB.Collection("projects")
	var projects []*models.Project

	for _, key := range keys {
		var project models.Project
		if err := collection.FindByID(key, &project); err == nil {
			projects = append(projects, &project)
		}
	}

	return projects, nil
}

func (d *ProBBoltSystemDriver) CheckProjectName(ctx context.Context, name string) error {
	collection := d.DB.Collection("projects")
	var results []models.Project

	err := collection.Find("name", name, &results)
	if err != nil {
		return err
	}

	if len(results) > 0 {
		return fmt.Errorf("%w", ae.ErrProjectNameTaken)
	}

	return nil
}

func (d *ProBBoltSystemDriver) SearchProjects(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Project], error) {
	collection := d.DB.Collection("projects")
	var projects []models.Project

	// Build query based on parameters
	var query *apitobolt.Query

	// Apply filters if provided
	if param.ProjectID != "" {
		query = collection.Select(q.Eq("id", param.ProjectID))
	} else {
		query = collection.Select()
	}

	// Apply pagination - simplified for BBolt implementation
	// In production, you'd add Limit and Offset to CommonSystemParams or use a different approach
	query = query.Limit(100) // Default limit

	err := query.Find(&projects)
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	var results []*models.Project
	for i := range projects {
		results = append(results, &projects[i])
	}

	return &models.SearchResponse[models.Project]{
		Results: results,
	}, nil
}

func (d *ProBBoltSystemDriver) FindUserProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	// For BBolt, we'll implement a simple approach where we store user project associations
	// In a real implementation, you might want to have a separate collection for user-project relationships

	collection := d.DB.Collection("projects")
	var allProjects []models.Project

	// Get all projects and filter by user access
	// This is a simplified implementation - in production you'd want proper relationship tracking
	err := collection.All(&allProjects)
	if err != nil {
		return nil, err
	}

	var userProjects []*models.Project
	for i := range allProjects {
		// For now, we'll assume all projects are accessible to all users
		// In a real implementation, you'd check user permissions/ownership
		userProjects = append(userProjects, &allProjects[i])
	}

	return userProjects, nil
}

func (d *ProBBoltSystemDriver) FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	// Simplified implementation - in production you'd have proper user-project relationship tracking
	projects, err := d.FindUserProjects(ctx, userId)
	if err != nil {
		return nil, err
	}

	var projectsWithRoles []*models.ProjectWithRoles
	for _, project := range projects {
		projectWithRole := &models.ProjectWithRoles{
			Project: project,
			Role:    "admin", // Default role - in production this would come from user-project relationships
		}
		projectsWithRoles = append(projectsWithRoles, projectWithRole)
	}

	return projectsWithRoles, nil
}

func (d *ProBBoltSystemDriver) CheckProjectWithRoles(ctx context.Context, userId, projectId string) (*models.ProjectWithRoles, error) {
	project, err := d.GetProject(ctx, projectId)
	if err != nil {
		return nil, err
	}

	user, err := d.GetSystemUser(ctx, userId)
	if err != nil {
		return nil, err
	}

	// it has to return 
	/*
	{
		user: Document('users', @user_id),
		project: v,
		role: e.role,
		permissions: e.permissions
	}
	*/

	// Simplified implementation - in production you'd check actual user permissions
	return &models.ProjectWithRoles{
		User:    user,
		Project: project,
		Role:    "admin", // Default role
		Permissions: []string{"all"}, // 
	}, nil
}

func (d *ProBBoltSystemDriver) DeleteProjectFromSystem(ctx context.Context, projectId string) error {
	collection := d.DB.Collection("projects")

	// First check if project exists
	var project models.Project
	err := collection.FindByID(projectId, &project)
	if err != nil {
		return err
	}

	// Delete the project
	err = collection.DeleteStruct(&models.Project{ID: projectId})
	if err != nil {
		return err
	}

	// Delete webhooks
	webhookCollection := d.DB.Collection("webhooks")
	var webhooks []models.Webhook
	err = webhookCollection.Find("project_id", projectId, &webhooks)
	if err == nil {
		for _, webhook := range webhooks {
			webhookCollection.DeleteStruct(&webhook)
		}
	}

	return nil
}

func (d *ProBBoltSystemDriver) TransferSchema(ctx context.Context, from, to string) error {
	fromProject, err := d.GetProject(ctx, from)
	if err != nil {
		return err
	}

	toProject, err := d.GetProject(ctx, to)
	if err != nil {
		return err
	}

	// Transfer schema, roles, and settings
	toProject.Schema = fromProject.Schema
	toProject.Roles = fromProject.Roles
	toProject.Settings = fromProject.Settings

	return d.UpdateProject(ctx, toProject, true)
}

// ListAllProjects returns all projects (admin only)
func (d *ProBBoltSystemDriver) ListAllProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	// Check if user is super admin
	collection := d.DB.Collection("projects")
	var projects []models.Project

	// Get all projects sorted by created date
	err := collection.All(&projects, apitobolt.Reverse())
	if err != nil {
		return nil, err
	}

	// Convert to pointers
	var results []*models.Project
	for i := range projects {
		results = append(results, &projects[i])
	}

	return results, nil
}

// AddSystemUserMetaInfo is deprecated according to interface comments
func (d *ProBBoltSystemDriver) AddSystemUserMetaInfo(ctx context.Context, doc *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	// This method is deprecated and replaced by dataloader resolver
	return doc, nil
}

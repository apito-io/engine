package couchbase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/couchbase/gocb/v2"
	"github.com/google/uuid"
)

// GetProject retrieves a project by ID using Couchbase
func (c *CouchbaseDriver) GetProject(ctx context.Context, id string) (*models.Project, error) {
	result, err := c.Collection.Get("project::"+id, nil)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := result.Content(&data); err != nil {
		return nil, err
	}

	if projectDataStr, ok := data["project_data"].(string); ok {
		var project models.Project
		if err := json.Unmarshal([]byte(projectDataStr), &project); err != nil {
			return nil, err
		}
		return &project, nil
	}

	return nil, fmt.Errorf("project data not found")
}

// GetSystemUser retrieves a system user by ID using Couchbase
func (c *CouchbaseDriver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
	result, err := c.Collection.Get("user::"+id, nil)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := result.Content(&data); err != nil {
		return nil, err
	}

	if userDataStr, ok := data["user_data"].(string); ok {
		var user models.SystemUser
		if err := json.Unmarshal([]byte(userDataStr), &user); err != nil {
			return nil, err
		}
		return &user, nil
	}

	return nil, fmt.Errorf("user data not found")
}

// GetSystemUserByEmail retrieves a system user by email using Couchbase
func (c *CouchbaseDriver) GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error) {
	query := fmt.Sprintf("SELECT user_data FROM `%s` WHERE doc_type = \"user\" AND email = $1 LIMIT 1", c.Bucket.Name())

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{email},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	if !results.Next() {
		return nil, fmt.Errorf("user not found")
	}

	var row map[string]interface{}
	if err := results.Row(&row); err != nil {
		return nil, err
	}

	if userDataStr, ok := row["user_data"].(string); ok {
		var user models.SystemUser
		if err := json.Unmarshal([]byte(userDataStr), &user); err != nil {
			return nil, err
		}
		return &user, nil
	}

	return nil, fmt.Errorf("user data not found")
}

// CheckProjectName checks if a project name already exists using Couchbase
func (c *CouchbaseDriver) CheckProjectName(ctx context.Context, name string) error {
	query := fmt.Sprintf("SELECT COUNT(*) as count FROM `%s` WHERE doc_type = \"project\" AND name = $1", c.Bucket.Name())

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{name},
	})
	if err != nil {
		return err
	}
	defer results.Close()

	if !results.Next() {
		return nil
	}

	var row map[string]interface{}
	if err := results.Row(&row); err != nil {
		return err
	}

	if count, ok := row["count"].(float64); ok && count > 0 {
		return fmt.Errorf("project already exists with this name")
	}

	return nil
}

// SearchProjects searches for projects based on common system parameters using Couchbase
func (c *CouchbaseDriver) SearchProjects(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Project], error) {
	var query string
	var args []interface{}

	if param.UserID != "" {
		// Search projects for a specific user
		query = fmt.Sprintf("SELECT project_data FROM `%s` WHERE doc_type = \"user_project\" AND user_id = $1 LIMIT 100", c.Bucket.Name())
		args = []interface{}{param.UserID}
	} else {
		// Search all projects
		query = fmt.Sprintf("SELECT project_data FROM `%s` WHERE doc_type = \"project\" LIMIT 100", c.Bucket.Name())
		args = []interface{}{}
	}

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: args,
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

	return &models.SearchResponse[models.Project]{
		Results: projects,
	}, nil
}

// SearchFunctions searches for cloud functions in a project using Couchbase
func (c *CouchbaseDriver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	project, err := c.GetProject(ctx, param.ProjectID)
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

// SearchWebHooks searches for webhooks in a project using Couchbase
func (c *CouchbaseDriver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	query := fmt.Sprintf("SELECT webhook_data FROM `%s` WHERE doc_type = \"webhook\" AND project_id = $1", c.Bucket.Name())

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{param.ProjectID},
	})
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var hooks []*models.Webhook
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if webhookDataStr, ok := row["webhook_data"].(string); ok {
			var hook models.Webhook
			if err := json.Unmarshal([]byte(webhookDataStr), &hook); err == nil {
				hooks = append(hooks, &hook)
			}
		}
	}

	return &models.SearchResponse[models.Webhook]{
		Results: hooks,
	}, nil
}

// GetWebHook retrieves a specific webhook by ID using Couchbase
func (c *CouchbaseDriver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	result, err := c.Collection.Get(fmt.Sprintf("webhook::%s::%s", projectId, hookId), nil)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := result.Content(&data); err != nil {
		return nil, err
	}

	if webhookDataStr, ok := data["webhook_data"].(string); ok {
		var webhook models.Webhook
		if err := json.Unmarshal([]byte(webhookDataStr), &webhook); err != nil {
			return nil, err
		}
		return &webhook, nil
	}

	return nil, fmt.Errorf("webhook data not found")
}

// DeleteWebhook deletes a webhook using Couchbase
func (c *CouchbaseDriver) DeleteWebhook(ctx context.Context, projectId, hookId string) error {
	_, err := c.Collection.Remove(fmt.Sprintf("webhook::%s::%s", projectId, hookId), nil)
	return err
}

// SearchUsers searches for system users based on parameters using Couchbase
func (c *CouchbaseDriver) SearchUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	query := fmt.Sprintf("SELECT user_data FROM `%s` WHERE doc_type = \"user\" LIMIT 100", c.Bucket.Name())

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var users []*models.SystemUser
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if userDataStr, ok := row["user_data"].(string); ok {
			var user models.SystemUser
			if err := json.Unmarshal([]byte(userDataStr), &user); err == nil {
				users = append(users, &user)
			}
		}
	}

	return &models.SearchResponse[models.SystemUser]{
		Results: users,
	}, nil
}

// AddSystemUserMetaInfo adds metadata to a system user using Couchbase
func (c *CouchbaseDriver) AddSystemUserMetaInfo(ctx context.Context, doc *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	metadataJson, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"id":       doc.ID,
		"metadata": string(metadataJson),
		"doc_type": "user_metadata",
	}

	_, err = c.Collection.Upsert("user_metadata::"+doc.ID, data, nil)
	return doc, err
}

// AddTeamMetaInfo adds metadata to team members using Couchbase
func (c *CouchbaseDriver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	for _, doc := range docs {
		metadataJson, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}

		data := map[string]interface{}{
			"id":       doc.ID,
			"metadata": string(metadataJson),
			"doc_type": "team_metadata",
		}

		_, err = c.Collection.Upsert("team_metadata::"+doc.ID, data, nil)
		if err != nil {
			return nil, err
		}
	}

	return docs, nil
}

// RemoveATeamMemberFromProject removes a team member from a project using Couchbase
func (c *CouchbaseDriver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberID string) error {
	// Remove from project_teams
	c.Collection.Remove(fmt.Sprintf("project_team::%s::%s", projectId, memberID), nil)

	// Remove from user_projects
	_, err := c.Collection.Remove(fmt.Sprintf("user_project::%s::%s", memberID, projectId), nil)
	return err
}

// CheckTeamMemberExists checks if a team member exists in a project using Couchbase
func (c *CouchbaseDriver) CheckTeamMemberExists(ctx context.Context, projectId string, memberID string) error {
	_, err := c.Collection.Get(fmt.Sprintf("project_team::%s::%s", projectId, memberID), nil)
	if err != nil {
		return fmt.Errorf("team member not found")
	}

	return nil
}

// CreateProject creates a new project using Couchbase
func (c *CouchbaseDriver) CreateProject(ctx context.Context, userId string, project *models.Project) (*models.Project, error) {
	if project.ID == "" {
		project.ID = uuid.New().String()
	}

	project.CreatedAt = time.Now().Format(time.RFC3339)
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	projectDataJson, err := json.Marshal(project)
	if err != nil {
		return nil, err
	}

	// Store project
	projectDoc := map[string]interface{}{
		"id":           project.ID,
		"name":         project.Name,
		"project_data": string(projectDataJson),
		"created_at":   project.CreatedAt,
		"updated_at":   project.UpdatedAt,
		"doc_type":     "project",
	}

	_, err = c.Collection.Upsert("project::"+project.ID, projectDoc, nil)
	if err != nil {
		return nil, err
	}

	// Create user-project relation
	userProjectDoc := map[string]interface{}{
		"user_id":      userId,
		"project_id":   project.ID,
		"project_data": string(projectDataJson),
		"role":         "owner",
		"permissions":  `["read", "write", "admin"]`,
		"doc_type":     "user_project",
	}

	_, err = c.Collection.Upsert(fmt.Sprintf("user_project::%s::%s", userId, project.ID), userProjectDoc, nil)
	return project, err
}

// CreateSystemUser creates a new system user using Couchbase
func (c *CouchbaseDriver) CreateSystemUser(ctx context.Context, user *models.SystemUser) (*models.SystemUser, error) {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	user.CreatedAt = time.Now().Format(time.RFC3339)
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	userDataJson, err := json.Marshal(user)
	if err != nil {
		return nil, err
	}

	doc := map[string]interface{}{
		"id":         user.ID,
		"email":      user.Email,
		"user_data":  string(userDataJson),
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
		"doc_type":   "user",
	}

	_, err = c.Collection.Upsert("user::"+user.ID, doc, nil)
	return user, err
}

// UpdateSystemUser updates a system user using Couchbase
func (c *CouchbaseDriver) UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error {
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	userDataJson, err := json.Marshal(user)
	if err != nil {
		return err
	}

	doc := map[string]interface{}{
		"id":         user.ID,
		"email":      user.Email,
		"user_data":  string(userDataJson),
		"updated_at": user.UpdatedAt,
		"doc_type":   "user",
	}

	_, err = c.Collection.Upsert("user::"+user.ID, doc, nil)
	return err
}

// UpdateProject updates a project using Couchbase
func (c *CouchbaseDriver) UpdateProject(ctx context.Context, project *models.Project, replace bool) error {
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	projectDataJson, err := json.Marshal(project)
	if err != nil {
		return err
	}

	doc := map[string]interface{}{
		"id":           project.ID,
		"name":         project.Name,
		"project_data": string(projectDataJson),
		"updated_at":   project.UpdatedAt,
		"doc_type":     "project",
	}

	_, err = c.Collection.Upsert("project::"+project.ID, doc, nil)
	return err
}

// CheckTokenBlacklisted checks if a token is blacklisted using Couchbase
func (c *CouchbaseDriver) CheckTokenBlacklisted(ctx context.Context, tokenId string) error {
	_, err := c.Collection.Get("token_blacklist::"+tokenId, nil)
	if err == nil {
		return fmt.Errorf("token is blacklisted")
	}

	return nil
}

// BlacklistAToken adds a token to the blacklist using Couchbase
func (c *CouchbaseDriver) BlacklistAToken(ctx context.Context, token map[string]interface{}) error {
	tokenId, exists := token["jti"].(string)
	if !exists {
		return fmt.Errorf("token ID not found")
	}

	tokenDataJson, err := json.Marshal(token)
	if err != nil {
		return err
	}

	doc := map[string]interface{}{
		"token_id":   tokenId,
		"token_data": string(tokenDataJson),
		"created_at": time.Now().Format(time.RFC3339),
		"doc_type":   "token_blacklist",
	}

	_, err = c.Collection.Upsert("token_blacklist::"+tokenId, doc, nil)
	return err
}

// DeleteProjectFromSystem deletes a project and all related data using Couchbase
func (c *CouchbaseDriver) DeleteProjectFromSystem(ctx context.Context, projectId string) error {
	// Delete user-project relations
	query1 := fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"user_project\" AND project_id = $1", c.Bucket.Name())
	_, err := c.Cluster.Query(query1, &gocb.QueryOptions{
		PositionalParameters: []interface{}{projectId},
	})
	if err != nil {
		return err
	}

	// Delete project team relations
	query2 := fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"project_team\" AND project_id = $1", c.Bucket.Name())
	_, err = c.Cluster.Query(query2, &gocb.QueryOptions{
		PositionalParameters: []interface{}{projectId},
	})
	if err != nil {
		return err
	}

	// Delete webhooks
	query3 := fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"webhook\" AND project_id = $1", c.Bucket.Name())
	_, err = c.Cluster.Query(query3, &gocb.QueryOptions{
		PositionalParameters: []interface{}{projectId},
	})
	if err != nil {
		return err
	}

	// Delete the project
	_, err = c.Collection.Remove("project::"+projectId, nil)
	return err
}

// AddWebhookToProject adds a webhook to a project using Couchbase
func (c *CouchbaseDriver) AddWebhookToProject(ctx context.Context, doc *models.Webhook) (*models.Webhook, error) {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	webhookDataJson, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	webhookDoc := map[string]interface{}{
		"project_id":   doc.ProjectID,
		"webhook_id":   doc.ID,
		"webhook_data": string(webhookDataJson),
		"doc_type":     "webhook",
	}

	_, err = c.Collection.Upsert(fmt.Sprintf("webhook::%s::%s", doc.ProjectID, doc.ID), webhookDoc, nil)
	return doc, err
}

// SaveRawData saves raw data using Couchbase for payment-related operations
func (c *CouchbaseDriver) SaveRawData(ctx context.Context, collection string, data map[string]interface{}) error {
	id := uuid.New().String()

	dataJson, err := json.Marshal(data)
	if err != nil {
		return err
	}

	doc := map[string]interface{}{
		"id":           id,
		"collection":   collection,
		"data_content": string(dataJson),
		"created_at":   time.Now().Format(time.RFC3339),
		"doc_type":     "raw_data",
	}

	_, err = c.Collection.Upsert("raw_data::"+id, doc, nil)
	return err
}

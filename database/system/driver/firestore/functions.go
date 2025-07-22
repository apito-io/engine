package firestore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/google/uuid"
	"google.golang.org/api/iterator"
)

// GetProject retrieves a project by ID using Firestore
func (f *FirestoreDriver) GetProject(ctx context.Context, id string) (*models.Project, error) {
	doc, err := f.Client.Collection("projects").Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := doc.DataTo(&data); err != nil {
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

// GetSystemUser retrieves a system user by ID using Firestore
func (f *FirestoreDriver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
	doc, err := f.Client.Collection("users").Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := doc.DataTo(&data); err != nil {
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

// GetSystemUserByEmail retrieves a system user by email using Firestore
func (f *FirestoreDriver) GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error) {
	iter := f.Client.Collection("users").Where("email", "==", email).Limit(1).Documents(ctx)
	defer iter.Stop()

	doc, err := iter.Next()
	if err == iterator.Done {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := doc.DataTo(&data); err != nil {
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

// CheckProjectName checks if a project name already exists using Firestore
func (f *FirestoreDriver) CheckProjectName(ctx context.Context, name string) error {
	iter := f.Client.Collection("projects").Where("name", "==", name).Limit(1).Documents(ctx)
	defer iter.Stop()

	_, err := iter.Next()
	if err == iterator.Done {
		return nil // Project name is available
	}
	if err != nil {
		return err
	}

	return fmt.Errorf("project already exists with this name")
}

// SearchProjects searches for projects based on common system parameters using Firestore
func (f *FirestoreDriver) SearchProjects(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Project], error) {
	var iter *firestore.DocumentIterator

	if param.UserID != "" {
		// Search projects for a specific user
		iter = f.Client.Collection("user_projects").Where("user_id", "==", param.UserID).Limit(100).Documents(ctx)
	} else {
		// Search all projects
		iter = f.Client.Collection("projects").Limit(100).Documents(ctx)
	}
	defer iter.Stop()

	var projects []*models.Project
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var data map[string]interface{}
		if err := doc.DataTo(&data); err != nil {
			continue
		}

		if projectDataStr, ok := data["project_data"].(string); ok {
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

// SearchFunctions searches for cloud functions in a project using Firestore
func (f *FirestoreDriver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	project, err := f.GetProject(ctx, param.ProjectID)
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

// SearchWebHooks searches for webhooks in a project using Firestore
func (f *FirestoreDriver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	iter := f.Client.Collection("webhooks").Where("project_id", "==", param.ProjectID).Documents(ctx)
	defer iter.Stop()

	var hooks []*models.Webhook
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var data map[string]interface{}
		if err := doc.DataTo(&data); err != nil {
			continue
		}

		if webhookDataStr, ok := data["webhook_data"].(string); ok {
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

// GetWebHook retrieves a specific webhook by ID using Firestore
func (f *FirestoreDriver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	doc, err := f.Client.Collection("webhooks").Doc(fmt.Sprintf("%s_%s", projectId, hookId)).Get(ctx)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := doc.DataTo(&data); err != nil {
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

// DeleteWebhook deletes a webhook using Firestore
func (f *FirestoreDriver) DeleteWebhook(ctx context.Context, projectId, hookId string) error {
	_, err := f.Client.Collection("webhooks").Doc(fmt.Sprintf("%s_%s", projectId, hookId)).Delete(ctx)
	return err
}

// SearchUsers searches for system users based on parameters using Firestore
func (f *FirestoreDriver) SearchUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	iter := f.Client.Collection("users").Limit(100).Documents(ctx)
	defer iter.Stop()

	var users []*models.SystemUser
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var data map[string]interface{}
		if err := doc.DataTo(&data); err != nil {
			continue
		}

		if userDataStr, ok := data["user_data"].(string); ok {
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

// AddSystemUserMetaInfo adds metadata to a system user using Firestore
func (f *FirestoreDriver) AddSystemUserMetaInfo(ctx context.Context, doc *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	metadataJson, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"id":       doc.ID,
		"metadata": string(metadataJson),
	}

	_, err = f.Client.Collection("user_metadata").Doc(doc.ID).Set(ctx, data)
	return doc, err
}

// AddTeamMetaInfo adds metadata to team members using Firestore
func (f *FirestoreDriver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	batch := f.Client.Batch()

	for _, doc := range docs {
		metadataJson, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}

		data := map[string]interface{}{
			"id":       doc.ID,
			"metadata": string(metadataJson),
		}

		ref := f.Client.Collection("team_metadata").Doc(doc.ID)
		batch.Set(ref, data)
	}

	_, err := batch.Commit(ctx)
	return docs, err
}

// RemoveATeamMemberFromProject removes a team member from a project using Firestore
func (f *FirestoreDriver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberID string) error {
	batch := f.Client.Batch()

	// Remove from project_teams collection
	projectTeamsRef := f.Client.Collection("project_teams").Doc(fmt.Sprintf("%s_%s", projectId, memberID))
	batch.Delete(projectTeamsRef)

	// Remove from user_projects collection
	userProjectsRef := f.Client.Collection("user_projects").Doc(fmt.Sprintf("%s_%s", memberID, projectId))
	batch.Delete(userProjectsRef)

	_, err := batch.Commit(ctx)
	return err
}

// CheckTeamMemberExists checks if a team member exists in a project using Firestore
func (f *FirestoreDriver) CheckTeamMemberExists(ctx context.Context, projectId string, memberID string) error {
	_, err := f.Client.Collection("project_teams").Doc(fmt.Sprintf("%s_%s", projectId, memberID)).Get(ctx)
	if err != nil {
		return fmt.Errorf("team member not found")
	}

	return nil
}

// CreateProject creates a new project using Firestore
func (f *FirestoreDriver) CreateProject(ctx context.Context, userId string, project *models.Project) (*models.Project, error) {
	if project.ID == "" {
		project.ID = uuid.New().String()
	}

	project.CreatedAt = time.Now().Format(time.RFC3339)
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	projectDataJson, err := json.Marshal(project)
	if err != nil {
		return nil, err
	}

	batch := f.Client.Batch()

	// Store project in projects collection
	projectData := map[string]interface{}{
		"id":           project.ID,
		"name":         project.Name,
		"project_data": string(projectDataJson),
		"created_at":   time.Now(),
		"updated_at":   time.Now(),
	}

	projectRef := f.Client.Collection("projects").Doc(project.ID)
	batch.Set(projectRef, projectData)

	// Create user-project relation
	userProjectData := map[string]interface{}{
		"user_id":      userId,
		"project_id":   project.ID,
		"project_data": string(projectDataJson),
		"role":         "owner",
		"permissions":  []string{"read", "write", "admin"},
	}

	userProjectRef := f.Client.Collection("user_projects").Doc(fmt.Sprintf("%s_%s", userId, project.ID))
	batch.Set(userProjectRef, userProjectData)

	_, err = batch.Commit(ctx)
	return project, err
}

// CreateSystemUser creates a new system user using Firestore
func (f *FirestoreDriver) CreateSystemUser(ctx context.Context, user *models.SystemUser) (*models.SystemUser, error) {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	user.CreatedAt = time.Now().Format(time.RFC3339)
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	userDataJson, err := json.Marshal(user)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"id":         user.ID,
		"email":      user.Email,
		"user_data":  string(userDataJson),
		"created_at": time.Now(),
		"updated_at": time.Now(),
	}

	_, err = f.Client.Collection("users").Doc(user.ID).Set(ctx, data)
	return user, err
}

// UpdateSystemUser updates a system user using Firestore
func (f *FirestoreDriver) UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error {
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	userDataJson, err := json.Marshal(user)
	if err != nil {
		return err
	}

	updates := []firestore.Update{
		{Path: "user_data", Value: string(userDataJson)},
		{Path: "updated_at", Value: time.Now()},
	}

	_, err = f.Client.Collection("users").Doc(user.ID).Update(ctx, updates)
	return err
}

// UpdateProject updates a project using Firestore
func (f *FirestoreDriver) UpdateProject(ctx context.Context, project *models.Project, replace bool) error {
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	projectDataJson, err := json.Marshal(project)
	if err != nil {
		return err
	}

	updates := []firestore.Update{
		{Path: "project_data", Value: string(projectDataJson)},
		{Path: "updated_at", Value: time.Now()},
	}

	_, err = f.Client.Collection("projects").Doc(project.ID).Update(ctx, updates)
	return err
}

// CheckTokenBlacklisted checks if a token is blacklisted using Firestore
func (f *FirestoreDriver) CheckTokenBlacklisted(ctx context.Context, tokenId string) error {
	_, err := f.Client.Collection("token_blacklist").Doc(tokenId).Get(ctx)
	if err == nil {
		return fmt.Errorf("token is blacklisted")
	}

	return nil
}

// BlacklistAToken adds a token to the blacklist using Firestore
func (f *FirestoreDriver) BlacklistAToken(ctx context.Context, token map[string]interface{}) error {
	tokenId, exists := token["jti"].(string)
	if !exists {
		return fmt.Errorf("token ID not found")
	}

	tokenDataJson, err := json.Marshal(token)
	if err != nil {
		return err
	}

	data := map[string]interface{}{
		"token_id":   tokenId,
		"token_data": string(tokenDataJson),
		"created_at": time.Now(),
	}

	_, err = f.Client.Collection("token_blacklist").Doc(tokenId).Set(ctx, data)
	return err
}

// DeleteProjectFromSystem deletes a project and all related data using Firestore
func (f *FirestoreDriver) DeleteProjectFromSystem(ctx context.Context, projectId string) error {
	batch := f.Client.Batch()

	// Delete user-project relations
	userProjectsIter := f.Client.Collection("user_projects").Where("project_id", "==", projectId).Documents(ctx)
	defer userProjectsIter.Stop()

	for {
		doc, err := userProjectsIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		batch.Delete(doc.Ref)
	}

	// Delete project team relations
	projectTeamsIter := f.Client.Collection("project_teams").Where("project_id", "==", projectId).Documents(ctx)
	defer projectTeamsIter.Stop()

	for {
		doc, err := projectTeamsIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		batch.Delete(doc.Ref)
	}

	// Delete webhooks
	webhooksIter := f.Client.Collection("webhooks").Where("project_id", "==", projectId).Documents(ctx)
	defer webhooksIter.Stop()

	for {
		doc, err := webhooksIter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		batch.Delete(doc.Ref)
	}

	// Delete the project
	projectRef := f.Client.Collection("projects").Doc(projectId)
	batch.Delete(projectRef)

	_, err := batch.Commit(ctx)
	return err
}

// AddWebhookToProject adds a webhook to a project using Firestore
func (f *FirestoreDriver) AddWebhookToProject(ctx context.Context, doc *models.Webhook) (*models.Webhook, error) {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	webhookDataJson, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"project_id":   doc.ProjectID,
		"webhook_id":   doc.ID,
		"webhook_data": string(webhookDataJson),
	}

	_, err = f.Client.Collection("webhooks").Doc(fmt.Sprintf("%s_%s", doc.ProjectID, doc.ID)).Set(ctx, data)
	return doc, err
}

// SaveRawData saves raw data using Firestore for payment-related operations
func (f *FirestoreDriver) SaveRawData(ctx context.Context, collection string, data map[string]interface{}) error {
	id := uuid.New().String()

	dataJson, err := json.Marshal(data)
	if err != nil {
		return err
	}

	docData := map[string]interface{}{
		"id":           id,
		"collection":   collection,
		"data_content": string(dataJson),
		"created_at":   time.Now(),
	}

	_, err = f.Client.Collection("raw_data").Doc(id).Set(ctx, docData)
	return err
}

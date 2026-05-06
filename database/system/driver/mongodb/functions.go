package mongodb

import (
	"context"
	"fmt"
	"time"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/database/system/driverdefaults"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// GetProject retrieves a project by ID using MongoDB
func (m *SystemMongoDriver) GetProject(ctx context.Context, id string) (*models.Project, error) {
	collection := m.Database.Collection("projects")

	var project models.Project
	err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&project)
	if err != nil {
		return nil, err
	}

	return &project, nil
}

// GetSystemUser retrieves a system user by ID using MongoDB
func (m *SystemMongoDriver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
	collection := m.Database.Collection("users")

	var user models.SystemUser
	err := collection.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetSystemUserByEmail retrieves a system user by email using MongoDB
func (m *SystemMongoDriver) GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error) {
	collection := m.Database.Collection("users")

	var user models.SystemUser
	err := collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// CheckProjectName checks if a project name already exists using MongoDB
func (m *SystemMongoDriver) CheckProjectName(ctx context.Context, name string) error {
	collection := m.Database.Collection("projects")

	count, err := collection.CountDocuments(ctx, bson.M{"name": name})
	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("%w", ae.ErrProjectNameTaken)
	}

	return nil
}

// SearchProjects searches for projects based on common system parameters using MongoDB
func (m *SystemMongoDriver) SearchProjects(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Project], error) {
	collection := m.Database.Collection("projects")

	filter := bson.M{}

	// Add filters based on parameters
	if param.UserID != "" {
		// Find projects where user has access via user_projects collection
		userProjectsCollection := m.Database.Collection("user_projects")
		cursor, err := userProjectsCollection.Find(ctx, bson.M{"user_id": param.UserID})
		if err != nil {
			return nil, err
		}

		var userProjects []map[string]interface{}
		if err = cursor.All(ctx, &userProjects); err != nil {
			return nil, err
		}

		var projectIds []string
		for _, up := range userProjects {
			if projectId, ok := up["project_id"].(string); ok {
				projectIds = append(projectIds, projectId)
			}
		}

		filter["_id"] = bson.M{"$in": projectIds}
	}

	opts := options.Find().SetLimit(100) // Default limit
	cursor, err := collection.Find(ctx, filter, opts)
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

	return &models.SearchResponse[models.Project]{
		Results: projects,
	}, nil
}

// SearchFunctions searches for cloud functions in a project using MongoDB
func (m *SystemMongoDriver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	project, err := m.GetProject(ctx, param.ProjectID)
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

// SearchWebHooks searches for webhooks in a project using MongoDB
func (m *SystemMongoDriver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	collection := m.Database.Collection("webhooks")

	filter := bson.M{"project_id": param.ProjectID}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var hooks []*models.Webhook
	for cursor.Next(ctx) {
		var hook models.Webhook
		if err := cursor.Decode(&hook); err != nil {
			return nil, err
		}
		hooks = append(hooks, &hook)
	}

	return &models.SearchResponse[models.Webhook]{
		Results: hooks,
	}, nil
}

// GetWebHook retrieves a specific webhook by ID using MongoDB
func (m *SystemMongoDriver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	collection := m.Database.Collection("webhooks")

	var webhook models.Webhook
	err := collection.FindOne(ctx, bson.M{
		"project_id": projectId,
		"_id":        hookId,
	}).Decode(&webhook)

	return &webhook, err
}

// DeleteWebhook deletes a webhook using MongoDB
func (m *SystemMongoDriver) DeleteWebhook(ctx context.Context, projectId, hookId string) error {
	collection := m.Database.Collection("webhooks")

	_, err := collection.DeleteOne(ctx, bson.M{
		"project_id": projectId,
		"_id":        hookId,
	})

	return err
}

// SearchUsers searches for system users based on parameters using MongoDB
func (m *SystemMongoDriver) SearchUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	collection := m.Database.Collection("users")

	filter := bson.M{}
	opts := options.Find().SetLimit(100) // Default limit

	cursor, err := collection.Find(ctx, filter, opts)
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

	return &models.SearchResponse[models.SystemUser]{
		Results: users,
	}, nil
}

// AddSystemUserMetaInfo adds metadata to a system user using MongoDB
func (m *SystemMongoDriver) AddSystemUserMetaInfo(ctx context.Context, doc *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	collection := m.Database.Collection("user_metadata")

	metadata := map[string]interface{}{
		"_id":  doc.ID,
		"data": doc,
	}

	_, err := collection.InsertOne(ctx, metadata)
	return doc, err
}

// AddTeamMetaInfo adds metadata to team members using MongoDB
func (m *SystemMongoDriver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	collection := m.Database.Collection("team_metadata")

	var documents []interface{}
	for _, doc := range docs {
		metadata := map[string]interface{}{
			"_id":  doc.ID,
			"data": doc,
		}
		documents = append(documents, metadata)
	}

	_, err := collection.InsertMany(ctx, documents)
	return docs, err
}

// RemoveATeamMemberFromProject removes a team member from a project using MongoDB
func (m *SystemMongoDriver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberID string) error {
	collection := m.Database.Collection("project_teams")

	_, err := collection.DeleteMany(ctx, bson.M{
		"project_id": projectId,
		"user_id":    memberID,
	})

	return err
}

// CheckTeamMemberExists checks if a team member exists in a project using MongoDB
func (m *SystemMongoDriver) CheckTeamMemberExists(ctx context.Context, projectId string, memberID string) error {
	collection := m.Database.Collection("project_teams")

	count, err := collection.CountDocuments(ctx, bson.M{
		"project_id": projectId,
		"user_id":    memberID,
	})

	if err != nil {
		return err
	}

	if count == 0 {
		return fmt.Errorf("team member not found")
	}

	return nil
}

// CreateProject creates a new project using MongoDB
func (m *SystemMongoDriver) CreateProject(ctx context.Context, userId string, project *models.Project) (*models.Project, error) {
	if project.ID == "" {
		project.ID = utility.NewID()
	}

	if project.Driver == nil {
		project.Driver = driverdefaults.OSSBootstrapProjectDriver(m.Conf, project.ID)
		if project.Driver == nil {
			return nil, fmt.Errorf("CreateProject: driver credentials are required for project %s", project.ID)
		}
	}
	if project.Driver != nil && project.Driver.ProjectID == "" {
		project.Driver.ProjectID = project.ID
	}

	project.CreatedAt = time.Now().Format(time.RFC3339)
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	collection := m.Database.Collection("projects")
	_, err := collection.InsertOne(ctx, project)
	if err != nil {
		return nil, err
	}

	// Create user-project relation
	userProjectsCollection := m.Database.Collection("user_projects")
	_, err = userProjectsCollection.InsertOne(ctx, map[string]interface{}{
		"user_id":     userId,
		"project_id":  project.ID,
		"role":        "owner",
		"permissions": []string{"read", "write", "admin"},
	})

	return project, err
}

// CreateSystemUser creates a new system user using MongoDB
func (m *SystemMongoDriver) CreateSystemUser(ctx context.Context, user *models.SystemUser) (*models.SystemUser, error) {
	if user.ID == "" {
		user.ID = utility.NewID()
	}

	user.CreatedAt = time.Now().Format(time.RFC3339)
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	collection := m.Database.Collection("users")
	_, err := collection.InsertOne(ctx, user)

	return user, err
}

// UpdateSystemUser updates a system user using MongoDB
func (m *SystemMongoDriver) UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error {
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	collection := m.Database.Collection("users")

	if replace {
		_, err := collection.ReplaceOne(ctx, bson.M{"_id": user.ID}, user)
		return err
	} else {
		update := bson.M{"$set": user}
		_, err := collection.UpdateOne(ctx, bson.M{"_id": user.ID}, update)
		return err
	}
}

// UpdateProject updates a project using MongoDB
func (m *SystemMongoDriver) UpdateProject(ctx context.Context, project *models.Project, replace bool) error {
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	collection := m.Database.Collection("projects")

	if replace {
		_, err := collection.ReplaceOne(ctx, bson.M{"_id": project.ID}, project)
		return err
	} else {
		update := bson.M{"$set": project}
		_, err := collection.UpdateOne(ctx, bson.M{"_id": project.ID}, update)
		return err
	}
}

// PersistProjectModelTypes is a no-op for MongoDB; schema is stored on the project document via UpdateProject.
func (m *SystemMongoDriver) PersistProjectModelTypes(ctx context.Context, projectID string, schemaModels []*models.ModelType) error {
	return nil
}

// TouchProjectUpdatedAt updates only the project document timestamp.
func (m *SystemMongoDriver) TouchProjectUpdatedAt(ctx context.Context, projectID string) error {
	if projectID == "" {
		return nil
	}
	collection := m.Database.Collection("projects")
	_, err := collection.UpdateOne(ctx, bson.M{"_id": projectID}, bson.M{
		"$set": bson.M{"updated_at": time.Now().Format(time.RFC3339)},
	})
	return err
}

// UpsertModelType merges one model into the project document and saves.
func (m *SystemMongoDriver) UpsertModelType(ctx context.Context, projectID string, mt *models.ModelType) error {
	if projectID == "" || mt == nil || mt.Name == "" {
		return nil
	}
	proj, err := m.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	if proj.Schema == nil {
		proj.Schema = &models.ProjectSchema{}
	}
	for i, mod := range proj.Schema.Models {
		if mod != nil && mod.Name == mt.Name {
			proj.Schema.Models[i] = mt
			return m.UpdateProject(ctx, proj, true)
		}
	}
	proj.Schema.Models = append(proj.Schema.Models, mt)
	return m.UpdateProject(ctx, proj, true)
}

// DeleteModelType removes one model from the embedded schema.
func (m *SystemMongoDriver) DeleteModelType(ctx context.Context, projectID, modelName string) error {
	if projectID == "" || modelName == "" {
		return nil
	}
	proj, err := m.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	if proj.Schema == nil {
		return nil
	}
	out := proj.Schema.Models[:0]
	for _, mod := range proj.Schema.Models {
		if mod == nil || mod.Name != modelName {
			out = append(out, mod)
		}
	}
	proj.Schema.Models = out
	return m.UpdateProject(ctx, proj, true)
}

// CheckTokenBlacklisted checks if a token is blacklisted using MongoDB
func (m *SystemMongoDriver) CheckTokenBlacklisted(ctx context.Context, tokenId string) error {
	collection := m.Database.Collection("token_blacklist")

	count, err := collection.CountDocuments(ctx, bson.M{"token_id": tokenId})
	if err != nil {
		return err
	}

	if count > 0 {
		return fmt.Errorf("token is blacklisted")
	}

	return nil
}

// BlacklistAToken adds a token to the blacklist using MongoDB
func (m *SystemMongoDriver) BlacklistAToken(ctx context.Context, token map[string]interface{}) error {
	tokenId, exists := token["jti"].(string)
	if !exists {
		return fmt.Errorf("token ID not found")
	}

	collection := m.Database.Collection("token_blacklist")
	_, err := collection.InsertOne(ctx, map[string]interface{}{
		"token_id": tokenId,
		"data":     token,
	})

	return err
}

// DeleteProjectFromSystem deletes a project and all related data using MongoDB
func (m *SystemMongoDriver) DeleteProjectFromSystem(ctx context.Context, projectId string) error {
	// Use a session for transaction
	session, err := m.Client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sc context.Context) (interface{}, error) {
		// Delete user-project relations
		userProjectsCollection := m.Database.Collection("user_projects")
		_, err := userProjectsCollection.DeleteMany(sc, bson.M{"project_id": projectId})
		if err != nil {
			return nil, err
		}

		// Delete project team relations
		projectTeamsCollection := m.Database.Collection("project_teams")
		_, err = projectTeamsCollection.DeleteMany(sc, bson.M{"project_id": projectId})
		if err != nil {
			return nil, err
		}

		// Delete webhooks
		webhooksCollection := m.Database.Collection("webhooks")
		_, err = webhooksCollection.DeleteMany(sc, bson.M{"project_id": projectId})
		if err != nil {
			return nil, err
		}

		// Delete the project
		projectsCollection := m.Database.Collection("projects")
		_, err = projectsCollection.DeleteOne(sc, bson.M{"_id": projectId})
		if err != nil {
			return nil, err
		}

		return nil, nil
	})

	return err
}

// AddWebhookToProject adds a webhook to a project using MongoDB
func (m *SystemMongoDriver) AddWebhookToProject(ctx context.Context, doc *models.Webhook) (*models.Webhook, error) {
	if doc.ID == "" {
		doc.ID = utility.NewID()
	}

	collection := m.Database.Collection("webhooks")
	_, err := collection.InsertOne(ctx, doc)

	return doc, err
}

// SaveRawData saves raw data using MongoDB for payment-related operations
func (m *SystemMongoDriver) SaveRawData(ctx context.Context, collection string, data map[string]interface{}) error {
	id := utility.NewID()

	coll := m.Database.Collection("raw_data")
	_, err := coll.InsertOne(ctx, map[string]interface{}{
		"_id":        id,
		"collection": collection,
		"data":       data,
	})

	return err
}

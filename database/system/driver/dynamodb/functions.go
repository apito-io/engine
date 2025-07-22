package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// GetProject retrieves a project by ID using DynamoDB
func (d *DynamoDBDriver) GetProject(ctx context.Context, id string) (*models.Project, error) {
	tableName := d.TablePrefix + "projects"

	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	}

	result, err := d.Client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("project not found")
	}

	var data map[string]interface{}
	err = attributevalue.UnmarshalMap(result.Item, &data)
	if err != nil {
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

// GetSystemUser retrieves a system user by ID using DynamoDB
func (d *DynamoDBDriver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
	tableName := d.TablePrefix + "users"

	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	}

	result, err := d.Client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("user not found")
	}

	var data map[string]interface{}
	err = attributevalue.UnmarshalMap(result.Item, &data)
	if err != nil {
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

// GetSystemUserByEmail retrieves a system user by email using DynamoDB
func (d *DynamoDBDriver) GetSystemUserByEmail(ctx context.Context, email string) (*models.SystemUser, error) {
	tableName := d.TablePrefix + "users"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("EmailIndex"),
		KeyConditionExpression: aws.String("email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":email": &types.AttributeValueMemberS{Value: email},
		},
		Limit: aws.Int32(1),
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	var data map[string]interface{}
	err = attributevalue.UnmarshalMap(result.Items[0], &data)
	if err != nil {
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

// CheckProjectName checks if a project name already exists using DynamoDB
func (d *DynamoDBDriver) CheckProjectName(ctx context.Context, name string) error {
	tableName := d.TablePrefix + "projects"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("NameIndex"),
		KeyConditionExpression: aws.String("#name = :name"),
		ExpressionAttributeNames: map[string]string{
			"#name": "name",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name": &types.AttributeValueMemberS{Value: name},
		},
		Limit: aws.Int32(1),
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return err
	}

	if len(result.Items) > 0 {
		return fmt.Errorf("project already exists with this name")
	}

	return nil
}

// SearchProjects searches for projects based on common system parameters using DynamoDB
func (d *DynamoDBDriver) SearchProjects(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Project], error) {
	var tableName string
	var input interface{}

	if param.UserID != "" {
		// Search projects for a specific user
		tableName = d.TablePrefix + "user_projects"
		input = &dynamodb.QueryInput{
			TableName:              aws.String(tableName),
			KeyConditionExpression: aws.String("user_id = :user_id"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":user_id": &types.AttributeValueMemberS{Value: param.UserID},
			},
			Limit: aws.Int32(100),
		}
	} else {
		// Search all projects
		tableName = d.TablePrefix + "projects"
		input = &dynamodb.ScanInput{
			TableName: aws.String(tableName),
			Limit:     aws.Int32(100),
		}
	}

	var projects []*models.Project

	if queryInput, ok := input.(*dynamodb.QueryInput); ok {
		result, err := d.Client.Query(ctx, queryInput)
		if err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			var data map[string]interface{}
			err := attributevalue.UnmarshalMap(item, &data)
			if err != nil {
				continue
			}

			if projectDataStr, ok := data["project_data"].(string); ok {
				var project models.Project
				if err := json.Unmarshal([]byte(projectDataStr), &project); err == nil {
					projects = append(projects, &project)
				}
			}
		}
	} else if scanInput, ok := input.(*dynamodb.ScanInput); ok {
		result, err := d.Client.Scan(ctx, scanInput)
		if err != nil {
			return nil, err
		}

		for _, item := range result.Items {
			var data map[string]interface{}
			err := attributevalue.UnmarshalMap(item, &data)
			if err != nil {
				continue
			}

			if projectDataStr, ok := data["project_data"].(string); ok {
				var project models.Project
				if err := json.Unmarshal([]byte(projectDataStr), &project); err == nil {
					projects = append(projects, &project)
				}
			}
		}
	}

	return &models.SearchResponse[models.Project]{
		Results: projects,
	}, nil
}

// SearchFunctions searches for cloud functions in a project using DynamoDB
func (d *DynamoDBDriver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	project, err := d.GetProject(ctx, param.ProjectID)
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

// SearchWebHooks searches for webhooks in a project using DynamoDB
func (d *DynamoDBDriver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	tableName := d.TablePrefix + "webhooks"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("project_id = :project_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":project_id": &types.AttributeValueMemberS{Value: param.ProjectID},
		},
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	var hooks []*models.Webhook
	for _, item := range result.Items {
		var data map[string]interface{}
		err := attributevalue.UnmarshalMap(item, &data)
		if err != nil {
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

// GetWebHook retrieves a specific webhook by ID using DynamoDB
func (d *DynamoDBDriver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	tableName := d.TablePrefix + "webhooks"

	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"project_id": &types.AttributeValueMemberS{Value: projectId},
			"webhook_id": &types.AttributeValueMemberS{Value: hookId},
		},
	}

	result, err := d.Client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("webhook not found")
	}

	var data map[string]interface{}
	err = attributevalue.UnmarshalMap(result.Item, &data)
	if err != nil {
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

// DeleteWebhook deletes a webhook using DynamoDB
func (d *DynamoDBDriver) DeleteWebhook(ctx context.Context, projectId, hookId string) error {
	tableName := d.TablePrefix + "webhooks"

	input := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"project_id": &types.AttributeValueMemberS{Value: projectId},
			"webhook_id": &types.AttributeValueMemberS{Value: hookId},
		},
	}

	_, err := d.Client.DeleteItem(ctx, input)
	return err
}

// SearchUsers searches for system users based on parameters using DynamoDB
func (d *DynamoDBDriver) SearchUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	tableName := d.TablePrefix + "users"

	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
		Limit:     aws.Int32(100),
	}

	result, err := d.Client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	var users []*models.SystemUser
	for _, item := range result.Items {
		var data map[string]interface{}
		err := attributevalue.UnmarshalMap(item, &data)
		if err != nil {
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

// AddSystemUserMetaInfo adds metadata to a system user using DynamoDB
func (d *DynamoDBDriver) AddSystemUserMetaInfo(ctx context.Context, doc *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	tableName := d.TablePrefix + "user_metadata"

	metadataJson, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	item := map[string]types.AttributeValue{
		"id":       &types.AttributeValueMemberS{Value: doc.ID},
		"metadata": &types.AttributeValueMemberS{Value: string(metadataJson)},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	}

	_, err = d.Client.PutItem(ctx, input)
	return doc, err
}

// AddTeamMetaInfo adds metadata to team members using DynamoDB
func (d *DynamoDBDriver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	tableName := d.TablePrefix + "team_metadata"

	for _, doc := range docs {
		metadataJson, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}

		item := map[string]types.AttributeValue{
			"id":       &types.AttributeValueMemberS{Value: doc.ID},
			"metadata": &types.AttributeValueMemberS{Value: string(metadataJson)},
		}

		input := &dynamodb.PutItemInput{
			TableName: aws.String(tableName),
			Item:      item,
		}

		_, err = d.Client.PutItem(ctx, input)
		if err != nil {
			return nil, err
		}
	}

	return docs, nil
}

// RemoveATeamMemberFromProject removes a team member from a project using DynamoDB
func (d *DynamoDBDriver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberID string) error {
	// Remove from project_teams table
	tableName1 := d.TablePrefix + "project_teams"
	input1 := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName1),
		Key: map[string]types.AttributeValue{
			"project_id": &types.AttributeValueMemberS{Value: projectId},
			"user_id":    &types.AttributeValueMemberS{Value: memberID},
		},
	}

	_, err := d.Client.DeleteItem(ctx, input1)
	if err != nil {
		return err
	}

	// Remove from user_projects table
	tableName2 := d.TablePrefix + "user_projects"
	input2 := &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName2),
		Key: map[string]types.AttributeValue{
			"user_id":    &types.AttributeValueMemberS{Value: memberID},
			"project_id": &types.AttributeValueMemberS{Value: projectId},
		},
	}

	_, err = d.Client.DeleteItem(ctx, input2)
	return err
}

// CheckTeamMemberExists checks if a team member exists in a project using DynamoDB
func (d *DynamoDBDriver) CheckTeamMemberExists(ctx context.Context, projectId string, memberID string) error {
	tableName := d.TablePrefix + "project_teams"

	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"project_id": &types.AttributeValueMemberS{Value: projectId},
			"user_id":    &types.AttributeValueMemberS{Value: memberID},
		},
	}

	result, err := d.Client.GetItem(ctx, input)
	if err != nil {
		return err
	}

	if result.Item == nil {
		return fmt.Errorf("team member not found")
	}

	return nil
}

// CreateProject creates a new project using DynamoDB
func (d *DynamoDBDriver) CreateProject(ctx context.Context, userId string, project *models.Project) (*models.Project, error) {
	if project.ID == "" {
		project.ID = uuid.New().String()
	}

	project.CreatedAt = time.Now().Format(time.RFC3339)
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	projectDataJson, err := json.Marshal(project)
	if err != nil {
		return nil, err
	}

	// Store project in projects table
	projectsTable := d.TablePrefix + "projects"
	projectItem := map[string]types.AttributeValue{
		"id":           &types.AttributeValueMemberS{Value: project.ID},
		"name":         &types.AttributeValueMemberS{Value: project.Name},
		"project_data": &types.AttributeValueMemberS{Value: string(projectDataJson)},
		"created_at":   &types.AttributeValueMemberS{Value: project.CreatedAt},
		"updated_at":   &types.AttributeValueMemberS{Value: project.UpdatedAt},
	}

	input1 := &dynamodb.PutItemInput{
		TableName: aws.String(projectsTable),
		Item:      projectItem,
	}

	_, err = d.Client.PutItem(ctx, input1)
	if err != nil {
		return nil, err
	}

	// Create user-project relation
	userProjectsTable := d.TablePrefix + "user_projects"
	permissionsData, _ := attributevalue.Marshal([]string{"read", "write", "admin"})

	userProjectItem := map[string]types.AttributeValue{
		"user_id":      &types.AttributeValueMemberS{Value: userId},
		"project_id":   &types.AttributeValueMemberS{Value: project.ID},
		"project_data": &types.AttributeValueMemberS{Value: string(projectDataJson)},
		"role":         &types.AttributeValueMemberS{Value: "owner"},
		"permissions":  permissionsData,
	}

	input2 := &dynamodb.PutItemInput{
		TableName: aws.String(userProjectsTable),
		Item:      userProjectItem,
	}

	_, err = d.Client.PutItem(ctx, input2)
	return project, err
}

// CreateSystemUser creates a new system user using DynamoDB
func (d *DynamoDBDriver) CreateSystemUser(ctx context.Context, user *models.SystemUser) (*models.SystemUser, error) {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	user.CreatedAt = time.Now().Format(time.RFC3339)
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	userDataJson, err := json.Marshal(user)
	if err != nil {
		return nil, err
	}

	tableName := d.TablePrefix + "users"
	item := map[string]types.AttributeValue{
		"id":         &types.AttributeValueMemberS{Value: user.ID},
		"email":      &types.AttributeValueMemberS{Value: user.Email},
		"user_data":  &types.AttributeValueMemberS{Value: string(userDataJson)},
		"created_at": &types.AttributeValueMemberS{Value: user.CreatedAt},
		"updated_at": &types.AttributeValueMemberS{Value: user.UpdatedAt},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	}

	_, err = d.Client.PutItem(ctx, input)
	return user, err
}

// UpdateSystemUser updates a system user using DynamoDB
func (d *DynamoDBDriver) UpdateSystemUser(ctx context.Context, user *models.SystemUser, replace bool) error {
	user.UpdatedAt = time.Now().Format(time.RFC3339)

	userDataJson, err := json.Marshal(user)
	if err != nil {
		return err
	}

	tableName := d.TablePrefix + "users"
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: user.ID},
		},
		UpdateExpression: aws.String("SET user_data = :user_data, updated_at = :updated_at"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":user_data":  &types.AttributeValueMemberS{Value: string(userDataJson)},
			":updated_at": &types.AttributeValueMemberS{Value: user.UpdatedAt},
		},
	}

	_, err = d.Client.UpdateItem(ctx, input)
	return err
}

// UpdateProject updates a project using DynamoDB
func (d *DynamoDBDriver) UpdateProject(ctx context.Context, project *models.Project, replace bool) error {
	project.UpdatedAt = time.Now().Format(time.RFC3339)

	projectDataJson, err := json.Marshal(project)
	if err != nil {
		return err
	}

	tableName := d.TablePrefix + "projects"
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: project.ID},
		},
		UpdateExpression: aws.String("SET project_data = :project_data, updated_at = :updated_at"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":project_data": &types.AttributeValueMemberS{Value: string(projectDataJson)},
			":updated_at":   &types.AttributeValueMemberS{Value: project.UpdatedAt},
		},
	}

	_, err = d.Client.UpdateItem(ctx, input)
	return err
}

// CheckTokenBlacklisted checks if a token is blacklisted using DynamoDB
func (d *DynamoDBDriver) CheckTokenBlacklisted(ctx context.Context, tokenId string) error {
	tableName := d.TablePrefix + "token_blacklist"

	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"token_id": &types.AttributeValueMemberS{Value: tokenId},
		},
	}

	result, err := d.Client.GetItem(ctx, input)
	if err != nil {
		return err
	}

	if result.Item != nil {
		return fmt.Errorf("token is blacklisted")
	}

	return nil
}

// BlacklistAToken adds a token to the blacklist using DynamoDB
func (d *DynamoDBDriver) BlacklistAToken(ctx context.Context, token map[string]interface{}) error {
	tokenId, exists := token["jti"].(string)
	if !exists {
		return fmt.Errorf("token ID not found")
	}

	tokenDataJson, err := json.Marshal(token)
	if err != nil {
		return err
	}

	tableName := d.TablePrefix + "token_blacklist"
	item := map[string]types.AttributeValue{
		"token_id":   &types.AttributeValueMemberS{Value: tokenId},
		"token_data": &types.AttributeValueMemberS{Value: string(tokenDataJson)},
		"created_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	}

	_, err = d.Client.PutItem(ctx, input)
	return err
}

// DeleteProjectFromSystem deletes a project and all related data using DynamoDB
func (d *DynamoDBDriver) DeleteProjectFromSystem(ctx context.Context, projectId string) error {
	// Delete user-project relations
	userProjectsTable := d.TablePrefix + "user_projects"
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(userProjectsTable),
		IndexName:              aws.String("ProjectIdIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":project_id": &types.AttributeValueMemberS{Value: projectId},
		},
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return err
	}

	for _, item := range result.Items {
		var data map[string]interface{}
		attributevalue.UnmarshalMap(item, &data)

		userId, _ := data["user_id"].(string)
		deleteInput := &dynamodb.DeleteItemInput{
			TableName: aws.String(userProjectsTable),
			Key: map[string]types.AttributeValue{
				"user_id":    &types.AttributeValueMemberS{Value: userId},
				"project_id": &types.AttributeValueMemberS{Value: projectId},
			},
		}
		d.Client.DeleteItem(ctx, deleteInput)
	}

	// Delete webhooks
	webhooksTable := d.TablePrefix + "webhooks"
	webhookInput := &dynamodb.QueryInput{
		TableName:              aws.String(webhooksTable),
		KeyConditionExpression: aws.String("project_id = :project_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":project_id": &types.AttributeValueMemberS{Value: projectId},
		},
	}

	webhookResult, err := d.Client.Query(ctx, webhookInput)
	if err != nil {
		return err
	}

	for _, item := range webhookResult.Items {
		var data map[string]interface{}
		attributevalue.UnmarshalMap(item, &data)

		webhookId, _ := data["webhook_id"].(string)
		deleteInput := &dynamodb.DeleteItemInput{
			TableName: aws.String(webhooksTable),
			Key: map[string]types.AttributeValue{
				"project_id": &types.AttributeValueMemberS{Value: projectId},
				"webhook_id": &types.AttributeValueMemberS{Value: webhookId},
			},
		}
		d.Client.DeleteItem(ctx, deleteInput)
	}

	// Delete the project
	projectsTable := d.TablePrefix + "projects"
	deleteProjectInput := &dynamodb.DeleteItemInput{
		TableName: aws.String(projectsTable),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: projectId},
		},
	}

	_, err = d.Client.DeleteItem(ctx, deleteProjectInput)
	return err
}

// AddWebhookToProject adds a webhook to a project using DynamoDB
func (d *DynamoDBDriver) AddWebhookToProject(ctx context.Context, doc *models.Webhook) (*models.Webhook, error) {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	webhookDataJson, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	tableName := d.TablePrefix + "webhooks"
	item := map[string]types.AttributeValue{
		"project_id":   &types.AttributeValueMemberS{Value: doc.ProjectID},
		"webhook_id":   &types.AttributeValueMemberS{Value: doc.ID},
		"webhook_data": &types.AttributeValueMemberS{Value: string(webhookDataJson)},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	}

	_, err = d.Client.PutItem(ctx, input)
	return doc, err
}

// SaveRawData saves raw data using DynamoDB for payment-related operations
func (d *DynamoDBDriver) SaveRawData(ctx context.Context, collection string, data map[string]interface{}) error {
	id := uuid.New().String()

	dataJson, err := json.Marshal(data)
	if err != nil {
		return err
	}

	tableName := d.TablePrefix + "raw_data"
	item := map[string]types.AttributeValue{
		"id":           &types.AttributeValueMemberS{Value: id},
		"collection":   &types.AttributeValueMemberS{Value: collection},
		"data_content": &types.AttributeValueMemberS{Value: string(dataJson)},
		"created_at":   &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	}

	_, err = d.Client.PutItem(ctx, input)
	return err
}

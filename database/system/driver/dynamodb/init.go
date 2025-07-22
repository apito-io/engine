package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

type DynamoDBDriver struct {
	Client           *dynamodb.Client
	TablePrefix      string
	DriverCredential *models.DriverCredentials
}

// GetTeams retrieves teams for a given user using DynamoDB
func (d *DynamoDBDriver) GetTeams(ctx context.Context, userId string) ([]*models.Team, error) {
	tableName := d.TablePrefix + "user_teams"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("user_id = :user_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":user_id": &types.AttributeValueMemberS{Value: userId},
		},
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	var teams []*models.Team
	for _, item := range result.Items {
		var teamData map[string]interface{}
		err := attributevalue.UnmarshalMap(item, &teamData)
		if err != nil {
			continue
		}

		if teamDataStr, ok := teamData["team_data"].(string); ok {
			var team models.Team
			if err := json.Unmarshal([]byte(teamDataStr), &team); err == nil {
				teams = append(teams, &team)
			}
		}
	}

	return teams, nil
}

// GetTeamsMembers retrieves team members for a project using DynamoDB
func (d *DynamoDBDriver) GetTeamsMembers(ctx context.Context, projectId string) ([]*models.SystemUser, error) {
	tableName := d.TablePrefix + "project_teams"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("project_id = :project_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":project_id": &types.AttributeValueMemberS{Value: projectId},
		},
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	var users []*models.SystemUser
	for _, item := range result.Items {
		var userData map[string]interface{}
		err := attributevalue.UnmarshalMap(item, &userData)
		if err != nil {
			continue
		}

		if userDataStr, ok := userData["user_data"].(string); ok {
			var user models.SystemUser
			if err := json.Unmarshal([]byte(userDataStr), &user); err == nil {
				users = append(users, &user)
			}
		}
	}

	return users, nil
}

// FindUserProjectsWithRoles retrieves user projects with their roles and permissions using DynamoDB
func (d *DynamoDBDriver) FindUserProjectsWithRoles(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	tableName := d.TablePrefix + "user_projects"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("user_id = :user_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":user_id": &types.AttributeValueMemberS{Value: userId},
		},
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	var projectWithRoles []*models.ProjectWithRoles

	for _, item := range result.Items {
		var data map[string]interface{}
		err := attributevalue.UnmarshalMap(item, &data)
		if err != nil {
			continue
		}

		projectId, _ := data["project_id"].(string)
		role, _ := data["role"].(string)

		// Get the project
		project, err := d.GetProject(ctx, projectId)
		if err != nil {
			continue
		}

		// Get the user
		user, err := d.GetSystemUser(ctx, userId)
		if err != nil {
			continue
		}

		var permissions []string
		if permissionsData, ok := data["permissions"].([]interface{}); ok {
			for _, perm := range permissionsData {
				if p, ok := perm.(string); ok {
					permissions = append(permissions, p)
				}
			}
		}

		projectWithRoles = append(projectWithRoles, &models.ProjectWithRoles{
			User:        user,
			Project:     project,
			Role:        role,
			Permissions: permissions,
		})
	}

	return projectWithRoles, nil
}

// SearchResource searches for resources based on common system parameters using DynamoDB
func (d *DynamoDBDriver) SearchResource(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[any], error) {
	// Generic search implementation - can be extended based on specific needs
	return &models.SearchResponse[any]{
		Results: []*any{},
	}, nil
}

// FindOrganizationAdmin retrieves the admin of an organization using DynamoDB
func (d *DynamoDBDriver) FindOrganizationAdmin(ctx context.Context, orgId string) (*models.SystemUser, error) {
	tableName := d.TablePrefix + "user_organizations"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("OrganizationRoleIndex"),
		KeyConditionExpression: aws.String("organization_id = :org_id AND #role = :role"),
		ExpressionAttributeNames: map[string]string{
			"#role": "role",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":org_id": &types.AttributeValueMemberS{Value: orgId},
			":role":   &types.AttributeValueMemberS{Value: "admin"},
		},
		Limit: aws.Int32(1),
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("organization admin not found")
	}

	var data map[string]interface{}
	err = attributevalue.UnmarshalMap(result.Items[0], &data)
	if err != nil {
		return nil, err
	}

	userId, _ := data["user_id"].(string)
	return d.GetSystemUser(ctx, userId)
}

// SaveAuditLog saves an audit log entry using DynamoDB
func (d *DynamoDBDriver) SaveAuditLog(ctx context.Context, auditLog *models.AuditLogs) error {
	if auditLog.ID == "" {
		auditLog.ID = uuid.New().String()
	}

	tableName := d.TablePrefix + "audit_logs"

	auditLogJson, err := json.Marshal(auditLog)
	if err != nil {
		return err
	}

	item := map[string]types.AttributeValue{
		"id":         &types.AttributeValueMemberS{Value: auditLog.ID},
		"user_id":    &types.AttributeValueMemberS{Value: auditLog.UserID},
		"project_id": &types.AttributeValueMemberS{Value: auditLog.ProjectID},
		"audit_data": &types.AttributeValueMemberS{Value: string(auditLogJson)},
		"created_at": &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	}

	_, err = d.Client.PutItem(ctx, input)
	return err
}

// SearchAuditLogs searches for audit logs based on common system parameters using DynamoDB
func (d *DynamoDBDriver) SearchAuditLogs(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.AuditLogs], error) {
	tableName := d.TablePrefix + "audit_logs"

	var input *dynamodb.QueryInput

	if param.UserID != "" {
		input = &dynamodb.QueryInput{
			TableName:              aws.String(tableName),
			IndexName:              aws.String("UserIdIndex"),
			KeyConditionExpression: aws.String("user_id = :user_id"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":user_id": &types.AttributeValueMemberS{Value: param.UserID},
			},
			ScanIndexForward: aws.Bool(false), // DESC order
			Limit:            aws.Int32(100),
		}
	} else if param.ProjectID != "" {
		input = &dynamodb.QueryInput{
			TableName:              aws.String(tableName),
			IndexName:              aws.String("ProjectIdIndex"),
			KeyConditionExpression: aws.String("project_id = :project_id"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":project_id": &types.AttributeValueMemberS{Value: param.ProjectID},
			},
			ScanIndexForward: aws.Bool(false), // DESC order
			Limit:            aws.Int32(100),
		}
	} else {
		// Scan all audit logs
		scanInput := &dynamodb.ScanInput{
			TableName: aws.String(tableName),
			Limit:     aws.Int32(100),
		}

		result, err := d.Client.Scan(ctx, scanInput)
		if err != nil {
			return nil, err
		}

		var logs []*models.AuditLogs
		for _, item := range result.Items {
			var data map[string]interface{}
			err := attributevalue.UnmarshalMap(item, &data)
			if err != nil {
				continue
			}

			if auditDataStr, ok := data["audit_data"].(string); ok {
				var log models.AuditLogs
				if err := json.Unmarshal([]byte(auditDataStr), &log); err == nil {
					logs = append(logs, &log)
				}
			}
		}

		return &models.SearchResponse[models.AuditLogs]{
			Results: logs,
		}, nil
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	var logs []*models.AuditLogs
	for _, item := range result.Items {
		var data map[string]interface{}
		err := attributevalue.UnmarshalMap(item, &data)
		if err != nil {
			continue
		}

		if auditDataStr, ok := data["audit_data"].(string); ok {
			var log models.AuditLogs
			if err := json.Unmarshal([]byte(auditDataStr), &log); err == nil {
				logs = append(logs, &log)
			}
		}
	}

	return &models.SearchResponse[models.AuditLogs]{
		Results: logs,
	}, nil
}

// GetOrganizations retrieves organizations for a given user using DynamoDB
func (d *DynamoDBDriver) GetOrganizations(ctx context.Context, userId string) (*models.SearchResponse[models.Organization], error) {
	tableName := d.TablePrefix + "user_organizations"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("user_id = :user_id"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":user_id": &types.AttributeValueMemberS{Value: userId},
		},
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	var organizations []*models.Organization
	for _, item := range result.Items {
		var data map[string]interface{}
		err := attributevalue.UnmarshalMap(item, &data)
		if err != nil {
			continue
		}

		if orgDataStr, ok := data["organization_data"].(string); ok {
			var org models.Organization
			if err := json.Unmarshal([]byte(orgDataStr), &org); err == nil {
				organizations = append(organizations, &org)
			}
		}
	}

	return &models.SearchResponse[models.Organization]{
		Results: organizations,
	}, nil
}

// RunMigration runs the database migrations for DynamoDB (creates necessary tables)
func (d *DynamoDBDriver) RunMigration(ctx context.Context) error {
	tables := []struct {
		name       string
		attributes []types.AttributeDefinition
		keySchema  []types.KeySchemaElement
		indexes    []types.GlobalSecondaryIndex
	}{
		{
			name: d.TablePrefix + "users",
			attributes: []types.AttributeDefinition{
				{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("email"), AttributeType: types.ScalarAttributeTypeS},
			},
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash},
			},
			indexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("EmailIndex"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("email"), KeyType: types.KeyTypeHash},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
					ProvisionedThroughput: &types.ProvisionedThroughput{
						ReadCapacityUnits:  aws.Int64(5),
						WriteCapacityUnits: aws.Int64(5),
					},
				},
			},
		},
		{
			name: d.TablePrefix + "projects",
			attributes: []types.AttributeDefinition{
				{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("name"), AttributeType: types.ScalarAttributeTypeS},
			},
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash},
			},
			indexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("NameIndex"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("name"), KeyType: types.KeyTypeHash},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
					ProvisionedThroughput: &types.ProvisionedThroughput{
						ReadCapacityUnits:  aws.Int64(5),
						WriteCapacityUnits: aws.Int64(5),
					},
				},
			},
		},
		{
			name: d.TablePrefix + "user_projects",
			attributes: []types.AttributeDefinition{
				{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("project_id"), AttributeType: types.ScalarAttributeTypeS},
			},
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("user_id"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeRange},
			},
		},
		{
			name: d.TablePrefix + "audit_logs",
			attributes: []types.AttributeDefinition{
				{AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("project_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("created_at"), AttributeType: types.ScalarAttributeTypeS},
			},
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("id"), KeyType: types.KeyTypeHash},
			},
			indexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("UserIdIndex"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("user_id"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("created_at"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
					ProvisionedThroughput: &types.ProvisionedThroughput{
						ReadCapacityUnits:  aws.Int64(5),
						WriteCapacityUnits: aws.Int64(5),
					},
				},
				{
					IndexName: aws.String("ProjectIdIndex"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("created_at"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
					ProvisionedThroughput: &types.ProvisionedThroughput{
						ReadCapacityUnits:  aws.Int64(5),
						WriteCapacityUnits: aws.Int64(5),
					},
				},
			},
		},
	}

	for _, table := range tables {
		input := &dynamodb.CreateTableInput{
			TableName:            aws.String(table.name),
			AttributeDefinitions: table.attributes,
			KeySchema:            table.keySchema,
			BillingMode:          types.BillingModeProvisioned,
			ProvisionedThroughput: &types.ProvisionedThroughput{
				ReadCapacityUnits:  aws.Int64(5),
				WriteCapacityUnits: aws.Int64(5),
			},
		}

		if len(table.indexes) > 0 {
			input.GlobalSecondaryIndexes = table.indexes
		}

		_, err := d.Client.CreateTable(ctx, input)
		if err != nil {
			// Check if table already exists
			var resourceInUseErr *types.ResourceInUseException
			if !aws.IsErrorAs(err, &resourceInUseErr) {
				return fmt.Errorf("failed to create table %s: %v", table.name, err)
			}
		}
	}

	return nil
}

// GetDynamoDBDriver creates a new DynamoDB system driver instance
func GetDynamoDBDriver(driverCredentials *models.DriverCredentials) (*DynamoDBDriver, error) {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(driverCredentials.Host), // Use Host field for AWS region
	)
	if err != nil {
		return nil, err
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	return &DynamoDBDriver{
		Client:           client,
		TablePrefix:      driverCredentials.Database + "_", // Use Database field as table prefix
		DriverCredential: driverCredentials,
	}, nil
}

package dynamodb

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type DynamoDBDriver struct {
	Client           *dynamodb.Client
	TablePrefix      string
	DriverCredential *models.DriverCredentials
}

// GetDynamoDBDriver creates a new DynamoDB project driver instance
func GetDynamoDBDriver(driverCredentials *models.DriverCredentials) (*DynamoDBDriver, error) {
	// Create AWS config with custom credentials
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			driverCredentials.AccessKey,
			driverCredentials.SecretKey,
			"",
		)),
		config.WithRegion("us-east-1"), // Default region, can be configured
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %v", err)
	}

	// Create DynamoDB client
	client := dynamodb.NewFromConfig(cfg)

	// Use database name as table prefix
	tablePrefix := driverCredentials.Database
	if tablePrefix == "" {
		tablePrefix = "apito_project"
	}

	driver := &DynamoDBDriver{
		Client:           client,
		TablePrefix:      tablePrefix,
		DriverCredential: driverCredentials,
	}

	// Initialize tables
	if err := driver.initializeTables(context.Background()); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %v", err)
	}

	return driver, nil
}

// initializeTables creates the necessary DynamoDB tables for project operations
func (d *DynamoDBDriver) initializeTables(ctx context.Context) error {
	tables := []struct {
		name      string
		keySchema []types.KeySchemaElement
		attrs     []types.AttributeDefinition
		gsi       []types.GlobalSecondaryIndex
	}{
		// Project documents table
		{
			name: d.TablePrefix + "_documents",
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("document_id"), KeyType: types.KeyTypeRange},
			},
			attrs: []types.AttributeDefinition{
				{AttributeName: aws.String("project_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("document_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("model_name"), AttributeType: types.ScalarAttributeTypeS},
			},
			gsi: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("ModelIndex"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("model_name"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		// Project relations table
		{
			name: d.TablePrefix + "_relations",
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("relation_id"), KeyType: types.KeyTypeRange},
			},
			attrs: []types.AttributeDefinition{
				{AttributeName: aws.String("project_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("relation_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("from_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("to_id"), AttributeType: types.ScalarAttributeTypeS},
			},
			gsi: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("FromIdIndex"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("from_id"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
				{
					IndexName: aws.String("ToIdIndex"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("to_id"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		// Project revisions table
		{
			name: d.TablePrefix + "_revisions",
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("document_id"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("revision_id"), KeyType: types.KeyTypeRange},
			},
			attrs: []types.AttributeDefinition{
				{AttributeName: aws.String("document_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("revision_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("project_id"), AttributeType: types.ScalarAttributeTypeS},
			},
			gsi: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("ProjectIdIndex"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("document_id"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		// Project builders table
		{
			name: d.TablePrefix + "_builders",
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("user_id"), KeyType: types.KeyTypeRange},
			},
			attrs: []types.AttributeDefinition{
				{AttributeName: aws.String("project_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
		// Project users table
		{
			name: d.TablePrefix + "_users",
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("user_id"), KeyType: types.KeyTypeRange},
			},
			attrs: []types.AttributeDefinition{
				{AttributeName: aws.String("project_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("user_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("email"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("phone"), AttributeType: types.ScalarAttributeTypeS},
			},
			gsi: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String("EmailIndex"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("email"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
				{
					IndexName: aws.String("PhoneIndex"),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String("phone"), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		// Project models metadata table
		{
			name: d.TablePrefix + "_models",
			keySchema: []types.KeySchemaElement{
				{AttributeName: aws.String("project_id"), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String("model_name"), KeyType: types.KeyTypeRange},
			},
			attrs: []types.AttributeDefinition{
				{AttributeName: aws.String("project_id"), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String("model_name"), AttributeType: types.ScalarAttributeTypeS},
			},
		},
	}

	for _, table := range tables {
		input := &dynamodb.CreateTableInput{
			TableName:            aws.String(table.name),
			KeySchema:            table.keySchema,
			AttributeDefinitions: table.attrs,
			BillingMode:          types.BillingModePayPerRequest,
		}

		if len(table.gsi) > 0 {
			input.GlobalSecondaryIndexes = table.gsi
		}

		_, err := d.Client.CreateTable(ctx, input)
		if err != nil {
			// Check if table already exists
			if _, ok := err.(*types.ResourceInUseException); !ok {
				return fmt.Errorf("failed to create table %s: %v", table.name, err)
			}
		}
	}

	return nil
}

// DeleteProject deletes a project and all related data
func (d *DynamoDBDriver) DeleteProject(ctx context.Context, projectID string) error {
	tables := []string{
		d.TablePrefix + "_documents",
		d.TablePrefix + "_relations",
		d.TablePrefix + "_revisions",
		d.TablePrefix + "_builders",
		d.TablePrefix + "_users",
		d.TablePrefix + "_models",
	}

	for _, tableName := range tables {
		// Query all items for this project
		queryInput := &dynamodb.QueryInput{
			TableName:              aws.String(tableName),
			KeyConditionExpression: aws.String("project_id = :project_id"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":project_id": &types.AttributeValueMemberS{Value: projectID},
			},
		}

		result, err := d.Client.Query(ctx, queryInput)
		if err != nil {
			continue // Skip if table doesn't exist or error occurs
		}

		// Delete all items
		for _, item := range result.Items {
			var projectIdVal, sortKeyVal string

			if pId, ok := item["project_id"]; ok {
				if s, ok := pId.(*types.AttributeValueMemberS); ok {
					projectIdVal = s.Value
				}
			}

			// Determine sort key based on table
			switch tableName {
			case d.TablePrefix + "_documents":
				if docId, ok := item["document_id"]; ok {
					if s, ok := docId.(*types.AttributeValueMemberS); ok {
						sortKeyVal = s.Value
					}
				}
			case d.TablePrefix + "_relations":
				if relId, ok := item["relation_id"]; ok {
					if s, ok := relId.(*types.AttributeValueMemberS); ok {
						sortKeyVal = s.Value
					}
				}
			case d.TablePrefix + "_builders":
				if userId, ok := item["user_id"]; ok {
					if s, ok := userId.(*types.AttributeValueMemberS); ok {
						sortKeyVal = s.Value
					}
				}
			case d.TablePrefix + "_users":
				if userId, ok := item["user_id"]; ok {
					if s, ok := userId.(*types.AttributeValueMemberS); ok {
						sortKeyVal = s.Value
					}
				}
			case d.TablePrefix + "_models":
				if modelName, ok := item["model_name"]; ok {
					if s, ok := modelName.(*types.AttributeValueMemberS); ok {
						sortKeyVal = s.Value
					}
				}
			}

			if projectIdVal != "" && sortKeyVal != "" {
				deleteInput := &dynamodb.DeleteItemInput{
					TableName: aws.String(tableName),
					Key: map[string]types.AttributeValue{
						"project_id": &types.AttributeValueMemberS{Value: projectIdVal},
					},
				}

				// Add sort key based on table
				switch tableName {
				case d.TablePrefix + "_documents":
					deleteInput.Key["document_id"] = &types.AttributeValueMemberS{Value: sortKeyVal}
				case d.TablePrefix + "_relations":
					deleteInput.Key["relation_id"] = &types.AttributeValueMemberS{Value: sortKeyVal}
				case d.TablePrefix + "_builders", d.TablePrefix + "_users":
					deleteInput.Key["user_id"] = &types.AttributeValueMemberS{Value: sortKeyVal}
				case d.TablePrefix + "_models":
					deleteInput.Key["model_name"] = &types.AttributeValueMemberS{Value: sortKeyVal}
				case d.TablePrefix + "_revisions":
					// For revisions, primary key is document_id + revision_id
					if docId, ok := item["document_id"]; ok {
						if s, ok := docId.(*types.AttributeValueMemberS); ok {
							deleteInput.Key = map[string]types.AttributeValue{
								"document_id": &types.AttributeValueMemberS{Value: s.Value},
								"revision_id": &types.AttributeValueMemberS{Value: sortKeyVal},
							}
						}
					}
				}

				d.Client.DeleteItem(ctx, deleteInput)
			}
		}
	}

	return nil
}

// TransferProject transfers a project from one user to another
func (d *DynamoDBDriver) TransferProject(ctx context.Context, userId, from, to string) error {
	// Update ownership in documents table
	tableName := d.TablePrefix + "_documents"

	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("project_id = :project_id"),
		FilterExpression:       aws.String("owner_id = :from_user"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":project_id": &types.AttributeValueMemberS{Value: userId},
			":from_user":  &types.AttributeValueMemberS{Value: from},
		},
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return err
	}

	// Update each document
	for _, item := range result.Items {
		if docId, ok := item["document_id"]; ok {
			if s, ok := docId.(*types.AttributeValueMemberS); ok {
				updateInput := &dynamodb.UpdateItemInput{
					TableName: aws.String(tableName),
					Key: map[string]types.AttributeValue{
						"project_id":  &types.AttributeValueMemberS{Value: userId},
						"document_id": &types.AttributeValueMemberS{Value: s.Value},
					},
					UpdateExpression: aws.String("SET owner_id = :to_user"),
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":to_user": &types.AttributeValueMemberS{Value: to},
					},
				}

				d.Client.UpdateItem(ctx, updateInput)
			}
		}
	}

	return nil
}

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
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// CheckCollectionExists checks if a collection exists in the project
func (d *DynamoDBDriver) CheckCollectionExists(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) (bool, error) {
	var tableName string
	var queryKey string

	if isRelationCollection {
		tableName = d.TablePrefix + "_relations"
		queryKey = "project_id"
	} else {
		tableName = d.TablePrefix + "_documents"
		queryKey = "project_id"
	}

	// Check if any documents exist for this project and model
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String(fmt.Sprintf("%s = :project_id", queryKey)),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
		},
		Limit: aws.Int32(1),
	}

	if !isRelationCollection {
		queryInput.IndexName = aws.String("ModelIndex")
		queryInput.KeyConditionExpression = aws.String("project_id = :project_id AND model_name = :model_name")
		queryInput.ExpressionAttributeValues[":model_name"] = &dynamodbtypes.AttributeValueMemberS{Value: param.Model.Name}
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return false, err
	}

	return len(result.Items) > 0, nil
}

// AddCollection creates metadata for a new collection (model)
func (d *DynamoDBDriver) AddCollection(ctx context.Context, param *models.CommonSystemParams) error {
	tableName := d.TablePrefix + "_models"

	modelDataJSON, err := json.Marshal(param.Model)
	if err != nil {
		return err
	}

	item := map[string]dynamodbtypes.AttributeValue{
		"project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
		"model_name": &dynamodbtypes.AttributeValueMemberS{Value: param.Model.Name},
		"model_data": &dynamodbtypes.AttributeValueMemberS{Value: string(modelDataJSON)},
		"created_at": &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	_, err = d.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})

	return err
}

// AddModel adds a new model to the project
func (d *DynamoDBDriver) AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error) {
	tableName := d.TablePrefix + "_models"

	modelDataJSON, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}

	item := map[string]dynamodbtypes.AttributeValue{
		"project_id": &dynamodbtypes.AttributeValueMemberS{Value: project.ID},
		"model_name": &dynamodbtypes.AttributeValueMemberS{Value: model.Name},
		"model_data": &dynamodbtypes.AttributeValueMemberS{Value: string(modelDataJSON)},
		"created_at": &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	_, err = d.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		return nil, err
	}

	// Update project schema
	if project.Schema == nil {
		project.Schema = &models.ProjectSchema{
			ProjectID: project.ID,
			Models:    []*models.ModelType{model},
		}
	} else {
		project.Schema.Models = append(project.Schema.Models, model)
	}

	return project.Schema, nil
}

// AddFieldToModel adds a new field to an existing model in the project
func (d *DynamoDBDriver) AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parent_field string) (*models.ModelType, error) {
	// Update the model schema in DynamoDB
	if parent_field == "" && isUpdate {
		param.Model.Fields = append(param.Model.Fields, param.FieldInfo)
	} else if parent_field != "" {
		for _, f := range param.Model.Fields {
			if f.Identifier == parent_field {
				subField := param.FieldInfo
				var found bool
				for i, s := range f.SubFieldInfo {
					if s.Identifier == param.FieldInfo.Identifier {
						f.SubFieldInfo[i] = subField
						found = true
						break
					}
				}
				if !found {
					subField.Serial = uint32(len(f.SubFieldInfo)) + 1
					f.SubFieldInfo = append(f.SubFieldInfo, subField)
				}
			}
		}
	}

	// Update model metadata in DynamoDB
	modelsTable := d.TablePrefix + "_models"
	modelDataJSON, err := json.Marshal(param.Model)
	if err != nil {
		return nil, err
	}

	_, err = d.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(modelsTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			"model_name": &dynamodbtypes.AttributeValueMemberS{Value: param.Model.Name},
		},
		UpdateExpression: aws.String("SET model_data = :model_data, updated_at = :updated_at"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":model_data": &dynamodbtypes.AttributeValueMemberS{Value: string(modelDataJSON)},
			":updated_at": &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})

	return param.Model, err
}

// RenameModel renames a model in the project
func (d *DynamoDBDriver) RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error {
	modelsTable := d.TablePrefix + "_models"

	// Get the existing model metadata
	getInput := &dynamodb.GetItemInput{
		TableName: aws.String(modelsTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id": &dynamodbtypes.AttributeValueMemberS{Value: project.ID},
			"model_name": &dynamodbtypes.AttributeValueMemberS{Value: modelName},
		},
	}

	result, err := d.Client.GetItem(ctx, getInput)
	if err != nil {
		return err
	}

	if result.Item == nil {
		return fmt.Errorf("model %s not found", modelName)
	}

	// Create new model record with new name
	newItem := make(map[string]dynamodbtypes.AttributeValue)
	for k, v := range result.Item {
		newItem[k] = v
	}
	newItem["model_name"] = &dynamodbtypes.AttributeValueMemberS{Value: newName}
	newItem["updated_at"] = &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)}

	_, err = d.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(modelsTable),
		Item:      newItem,
	})
	if err != nil {
		return err
	}

	// Update all documents that use this model
	documentsTable := d.TablePrefix + "_documents"
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(documentsTable),
		IndexName:              aws.String("ModelIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND model_name = :model_name"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: project.ID},
			":model_name": &dynamodbtypes.AttributeValueMemberS{Value: modelName},
		},
	}

	queryResult, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return err
	}

	// Update each document's model_name
	for _, item := range queryResult.Items {
		if docId, ok := item["document_id"]; ok {
			if s, ok := docId.(*dynamodbtypes.AttributeValueMemberS); ok {
				_, err = d.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
					TableName: aws.String(documentsTable),
					Key: map[string]dynamodbtypes.AttributeValue{
						"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: project.ID},
						"document_id": &dynamodbtypes.AttributeValueMemberS{Value: s.Value},
					},
					UpdateExpression: aws.String("SET model_name = :new_name"),
					ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
						":new_name": &dynamodbtypes.AttributeValueMemberS{Value: newName},
					},
				})
				if err != nil {
					return err
				}
			}
		}
	}

	// Delete old model record
	_, err = d.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(modelsTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id": &dynamodbtypes.AttributeValueMemberS{Value: project.ID},
			"model_name": &dynamodbtypes.AttributeValueMemberS{Value: modelName},
		},
	})

	return err
}

// ConvertModel converts a model in the project
func (d *DynamoDBDriver) ConvertModel(ctx context.Context, project *models.Project, modelName string) error {
	// Model conversion logic - implementation specific to business needs
	// For now, return nil as this is a complex operation
	return nil
}

// DropModel drops a model from the project
func (d *DynamoDBDriver) DropModel(ctx context.Context, project *models.Project, modelName string) error {
	// First delete all documents of this model
	documentsTable := d.TablePrefix + "_documents"
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(documentsTable),
		IndexName:              aws.String("ModelIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND model_name = :model_name"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: project.ID},
			":model_name": &dynamodbtypes.AttributeValueMemberS{Value: modelName},
		},
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return err
	}

	// Delete each document
	for _, item := range result.Items {
		if docId, ok := item["document_id"]; ok {
			if s, ok := docId.(*dynamodbtypes.AttributeValueMemberS); ok {
				_, err = d.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
					TableName: aws.String(documentsTable),
					Key: map[string]dynamodbtypes.AttributeValue{
						"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: project.ID},
						"document_id": &dynamodbtypes.AttributeValueMemberS{Value: s.Value},
					},
				})
				if err != nil {
					return err
				}
			}
		}
	}

	// Delete model metadata
	modelsTable := d.TablePrefix + "_models"
	_, err = d.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(modelsTable),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id": &dynamodbtypes.AttributeValueMemberS{Value: project.ID},
			"model_name": &dynamodbtypes.AttributeValueMemberS{Value: modelName},
		},
	})

	return err
}

// CreateIndex creates an index for a model in the project
func (d *DynamoDBDriver) CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parent_field string) error {
	// DynamoDB indexes are managed at table level and defined during table creation
	// For project-specific indexes, we would need to create GSIs which is complex
	// For now, we'll just return nil as the basic indexes are already created
	return nil
}

// DropIndex drops an index from a model in the project
func (d *DynamoDBDriver) DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error {
	// DynamoDB index management requires table modification which is complex
	// For now, we'll just return nil
	return nil
}

// GetSingleProjectDocument retrieves a single project document by ID
func (d *DynamoDBDriver) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	tableName := d.TablePrefix + "_documents"

	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			"document_id": &dynamodbtypes.AttributeValueMemberS{Value: param.DocumentID},
		},
	}

	result, err := d.Client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("document not found")
	}

	var data map[string]interface{}
	err = attributevalue.UnmarshalMap(result.Item, &data)
	if err != nil {
		return nil, err
	}

	if documentDataStr, ok := data["document_data"].(string); ok {
		var document types.DefaultDocumentStructure
		if err := json.Unmarshal([]byte(documentDataStr), &document); err != nil {
			return nil, err
		}
		return &document, nil
	}

	return nil, fmt.Errorf("document data not found")
}

// GetSingleProjectDocumentBytes retrieves a single project document by ID as bytes
func (d *DynamoDBDriver) GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	tableName := d.TablePrefix + "_documents"

	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			"document_id": &dynamodbtypes.AttributeValueMemberS{Value: param.DocumentID},
		},
	}

	result, err := d.Client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("document not found")
	}

	var data map[string]interface{}
	err = attributevalue.UnmarshalMap(result.Item, &data)
	if err != nil {
		return nil, err
	}

	if documentDataStr, ok := data["document_data"].(string); ok {
		return []byte(documentDataStr), nil
	}

	return nil, fmt.Errorf("document data not found")
}

// GetSingleProjectDocumentRevisions retrieves the revision history of a single project document by ID
func (d *DynamoDBDriver) GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*models.DocumentRevisionHistory, error) {
	tableName := d.TablePrefix + "_revisions"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("document_id = :document_id"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":document_id": &dynamodbtypes.AttributeValueMemberS{Value: param.DocumentID},
		},
		ScanIndexForward: aws.Bool(false), // Newest first
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	var revisions []*models.DocumentRevisionHistory
	for _, item := range result.Items {
		var data map[string]interface{}
		err = attributevalue.UnmarshalMap(item, &data)
		if err != nil {
			continue
		}

		if revisionDataStr, ok := data["revision_data"].(string); ok {
			var revision models.DocumentRevisionHistory
			if err := json.Unmarshal([]byte(revisionDataStr), &revision); err == nil {
				revisions = append(revisions, &revision)
			}
		}
	}

	return revisions, nil
}

// GetSingleRawDocumentFromProject retrieves a single raw document from the project
func (d *DynamoDBDriver) GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	tableName := d.TablePrefix + "_documents"

	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			"document_id": &dynamodbtypes.AttributeValueMemberS{Value: param.DocumentID},
		},
	}

	result, err := d.Client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("document not found")
	}

	var data map[string]interface{}
	err = attributevalue.UnmarshalMap(result.Item, &data)
	if err != nil {
		return nil, err
	}

	if documentDataStr, ok := data["document_data"].(string); ok {
		var rawDocument interface{}
		if err := json.Unmarshal([]byte(documentDataStr), &rawDocument); err != nil {
			return nil, err
		}
		return rawDocument, nil
	}

	return nil, fmt.Errorf("document data not found")
}

// QueryMultiDocumentOfProject queries multiple documents in the project and returns the result as a slice of DefaultDocumentStructure
func (d *DynamoDBDriver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
	tableName := d.TablePrefix + "_documents"

	input := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("ModelIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND model_name = :model_name"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":model_name": &dynamodbtypes.AttributeValueMemberS{Value: param.Model.Name},
		},
		Limit: aws.Int32(100),
	}

	result, err := d.Client.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	var documents []*types.DefaultDocumentStructure
	for _, item := range result.Items {
		var data map[string]interface{}
		err = attributevalue.UnmarshalMap(item, &data)
		if err != nil {
			continue
		}

		if documentDataStr, ok := data["document_data"].(string); ok {
			var document types.DefaultDocumentStructure
			if err := json.Unmarshal([]byte(documentDataStr), &document); err == nil {
				documents = append(documents, &document)
			}
		}
	}

	return documents, nil
}

// QueryMultiDocumentOfProjectBytes queries multiple documents in the project and returns the result as bytes
func (d *DynamoDBDriver) QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	documents, err := d.QueryMultiDocumentOfProject(ctx, param)
	if err != nil {
		return nil, err
	}

	return json.Marshal(documents)
}

// AddDocumentToProject adds a new document to the project
func (d *DynamoDBDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
	tableName := d.TablePrefix + "_documents"

	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	// Initialize Meta field if nil
	if doc.Meta == nil {
		doc.Meta = &types.MetaField{}
	}

	doc.Meta.CreatedAt = time.Now().Format(time.RFC3339)
	doc.Meta.UpdatedAt = time.Now().Format(time.RFC3339)

	documentDataJSON, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	item := map[string]dynamodbtypes.AttributeValue{
		"project_id":    &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
		"document_id":   &dynamodbtypes.AttributeValueMemberS{Value: doc.ID},
		"model_name":    &dynamodbtypes.AttributeValueMemberS{Value: param.Model.Name},
		"document_data": &dynamodbtypes.AttributeValueMemberS{Value: string(documentDataJSON)},
		"created_at":    &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		"updated_at":    &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	_, err = d.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})

	return doc, err
}

// UpdateDocumentOfProject updates a particular document in the project
func (d *DynamoDBDriver) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {
	tableName := d.TablePrefix + "_documents"

	// Initialize Meta field if nil
	if doc.Meta == nil {
		doc.Meta = &types.MetaField{}
	}

	doc.Meta.UpdatedAt = time.Now().Format(time.RFC3339)

	documentDataJSON, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	_, err = d.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			"document_id": &dynamodbtypes.AttributeValueMemberS{Value: doc.ID},
		},
		UpdateExpression: aws.String("SET document_data = :document_data, updated_at = :updated_at"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":document_data": &dynamodbtypes.AttributeValueMemberS{Value: string(documentDataJSON)},
			":updated_at":    &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})

	return err
}

// DeleteDocumentFromProject deletes a document from the project
func (d *DynamoDBDriver) DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	tableName := d.TablePrefix + "_documents"

	_, err := d.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			"document_id": &dynamodbtypes.AttributeValueMemberS{Value: param.DocumentID},
		},
	})

	return err
}

// DeleteDocumentsFromProject deletes multiple documents from the project
func (d *DynamoDBDriver) DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	tableName := d.TablePrefix + "_documents"

	for _, docID := range param.DocumentIDs {
		_, err := d.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(tableName),
			Key: map[string]dynamodbtypes.AttributeValue{
				"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
				"document_id": &dynamodbtypes.AttributeValueMemberS{Value: docID},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteDocumentRelation deletes all relations or data in pivot tables from the project
func (d *DynamoDBDriver) DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error {
	relationsTable := d.TablePrefix + "_relations"

	// Delete relations where this document is the source
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(relationsTable),
		IndexName:              aws.String("FromIdIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND from_id = :from_id"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":from_id":    &dynamodbtypes.AttributeValueMemberS{Value: param.DocumentID},
		},
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return err
	}

	for _, item := range result.Items {
		var data map[string]interface{}
		attributevalue.UnmarshalMap(item, &data)

		relationId, _ := data["relation_id"].(string)
		_, err = d.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(relationsTable),
			Key: map[string]dynamodbtypes.AttributeValue{
				"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
				"relation_id": &dynamodbtypes.AttributeValueMemberS{Value: relationId},
			},
		})
		if err != nil {
			return err
		}
	}

	// Delete relations where this document is the target
	queryInput2 := &dynamodb.QueryInput{
		TableName:              aws.String(relationsTable),
		IndexName:              aws.String("ToIdIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND to_id = :to_id"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":to_id":      &dynamodbtypes.AttributeValueMemberS{Value: param.DocumentID},
		},
	}

	result2, err := d.Client.Query(ctx, queryInput2)
	if err != nil {
		return err
	}

	for _, item := range result2.Items {
		var data map[string]interface{}
		attributevalue.UnmarshalMap(item, &data)

		relationId, _ := data["relation_id"].(string)
		_, err = d.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(relationsTable),
			Key: map[string]dynamodbtypes.AttributeValue{
				"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
				"relation_id": &dynamodbtypes.AttributeValueMemberS{Value: relationId},
			},
		})
		if err != nil {
			return err
		}
	}

	return nil
}

package dynamodb

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamodbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

// AddRelationFields adds relation fields to a model (no-op for DynamoDB)
func (d *DynamoDBDriver) AddRelationFields(ctx context.Context, param *models.CommonSystemParams, sourceModel, targetModel, relationType string) error {
	// DynamoDB handles relations through separate relation documents, no schema changes needed
	return nil
}

// DeleteRelationDocuments deletes relation documents for a specific relation
func (d *DynamoDBDriver) DeleteRelationDocuments(ctx context.Context, param *models.CommonSystemParams, relationshipName string) error {
	tableName := d.TablePrefix + "_relations"

	// Query for all relations of this type
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("project_id = :project_id"),
		FilterExpression:       aws.String("relation_name = :relation_name"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id":    &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":relation_name": &dynamodbtypes.AttributeValueMemberS{Value: relationshipName},
		},
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return err
	}

	// Delete each relation document
	for _, item := range result.Items {
		if relationId, ok := item["relation_id"]; ok {
			if s, ok := relationId.(*dynamodbtypes.AttributeValueMemberS); ok {
				_, err = d.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
					TableName: aws.String(tableName),
					Key: map[string]dynamodbtypes.AttributeValue{
						"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
						"relation_id": &dynamodbtypes.AttributeValueMemberS{Value: s.Value},
					},
				})
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// GetRelationDocument retrieves a relation document
func (d *DynamoDBDriver) GetRelationDocument(ctx context.Context, cd *models.ConnectDisconnectParam) (*types.DefaultDocumentStructure, error) {
	tableName := d.TablePrefix + "_relations"

	// Query for the specific relation
	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: cd.DocCollectionName}, // project_id
			"relation_id": &dynamodbtypes.AttributeValueMemberS{Value: cd.CurrentActionID},   // relation_id
		},
	}

	result, err := d.Client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("relation document not found")
	}

	// Parse the result into DefaultDocumentStructure
	var doc types.DefaultDocumentStructure
	if dataAttr, ok := result.Item["relation_data"]; ok {
		if s, ok := dataAttr.(*dynamodbtypes.AttributeValueMemberS); ok {
			err = json.Unmarshal([]byte(s.Value), &doc)
			if err != nil {
				return nil, err
			}
		}
	}

	return &doc, nil
}

// CreateRelation creates a new relation between documents
func (d *DynamoDBDriver) CreateRelation(ctx context.Context, param *models.CommonSystemParams, relation *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	tableName := d.TablePrefix + "_relations"

	// Generate unique relation ID if not provided
	if relation.ID == "" {
		relation.ID = uuid.New().String()
	}

	relationDataJSON, err := json.Marshal(relation)
	if err != nil {
		return nil, err
	}

	item := map[string]dynamodbtypes.AttributeValue{
		"project_id":    &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
		"relation_id":   &dynamodbtypes.AttributeValueMemberS{Value: relation.ID},
		"relation_data": &dynamodbtypes.AttributeValueMemberS{Value: string(relationDataJSON)},
		"created_at":    &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	// Add from_id and to_id for GSI queries if available
	if relation.Data != nil {
		if fromId, ok := relation.Data["from_id"]; ok {
			item["from_id"] = &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("%v", fromId)}
		}
		if toId, ok := relation.Data["to_id"]; ok {
			item["to_id"] = &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("%v", toId)}
		}
		if relationName, ok := relation.Data["relation_name"]; ok {
			item["relation_name"] = &dynamodbtypes.AttributeValueMemberS{Value: fmt.Sprintf("%v", relationName)}
		}
	}

	_, err = d.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})

	return relation, err
}

// DeleteRelation deletes a specific relation
func (d *DynamoDBDriver) DeleteRelation(ctx context.Context, param *models.CommonSystemParams, relationID string) error {
	tableName := d.TablePrefix + "_relations"

	_, err := d.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			"relation_id": &dynamodbtypes.AttributeValueMemberS{Value: relationID},
		},
	})

	return err
}

// NewInsertableRelations creates new insertable relations
func (d *DynamoDBDriver) NewInsertableRelations(ctx context.Context, param *models.CommonSystemParams, relations []*types.DefaultDocumentStructure) ([]*types.DefaultDocumentStructure, error) {
	var createdRelations []*types.DefaultDocumentStructure

	for _, relation := range relations {
		createdRelation, err := d.CreateRelation(ctx, param, relation)
		if err != nil {
			return nil, err
		}
		createdRelations = append(createdRelations, createdRelation)
	}

	return createdRelations, nil
}

// CheckOneToOneRelationExists checks if a one-to-one relation already exists
func (d *DynamoDBDriver) CheckOneToOneRelationExists(ctx context.Context, param *models.CommonSystemParams, fromId, toId, relationName string) (bool, error) {
	tableName := d.TablePrefix + "_relations"

	// Query using FromIdIndex
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("FromIdIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND from_id = :from_id"),
		FilterExpression:       aws.String("to_id = :to_id AND relation_name = :relation_name"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id":    &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":from_id":       &dynamodbtypes.AttributeValueMemberS{Value: fromId},
			":to_id":         &dynamodbtypes.AttributeValueMemberS{Value: toId},
			":relation_name": &dynamodbtypes.AttributeValueMemberS{Value: relationName},
		},
		Limit: aws.Int32(1),
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return false, err
	}

	return len(result.Items) > 0, nil
}

// GetRelationIds gets relation IDs for a document
func (d *DynamoDBDriver) GetRelationIds(ctx context.Context, param *models.CommonSystemParams, documentID, relationName string) ([]string, error) {
	tableName := d.TablePrefix + "_relations"
	var relationIds []string

	// Query using FromIdIndex
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("FromIdIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND from_id = :from_id"),
		FilterExpression:       aws.String("relation_name = :relation_name"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id":    &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":from_id":       &dynamodbtypes.AttributeValueMemberS{Value: documentID},
			":relation_name": &dynamodbtypes.AttributeValueMemberS{Value: relationName},
		},
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return nil, err
	}

	for _, item := range result.Items {
		if toId, ok := item["to_id"]; ok {
			if s, ok := toId.(*dynamodbtypes.AttributeValueMemberS); ok {
				relationIds = append(relationIds, s.Value)
			}
		}
	}

	return relationIds, nil
}

// ConnectBuilder connects a builder to the project
func (d *DynamoDBDriver) ConnectBuilder(ctx context.Context, projectId, userId string) error {
	tableName := d.TablePrefix + "_builders"

	item := map[string]dynamodbtypes.AttributeValue{
		"project_id":   &dynamodbtypes.AttributeValueMemberS{Value: projectId},
		"user_id":      &dynamodbtypes.AttributeValueMemberS{Value: userId},
		"connected_at": &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
	}

	_, err := d.Client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})

	return err
}

// DisconnectBuilder disconnects a builder from the project
func (d *DynamoDBDriver) DisconnectBuilder(ctx context.Context, projectId, userId string) error {
	tableName := d.TablePrefix + "_builders"

	_, err := d.Client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id": &dynamodbtypes.AttributeValueMemberS{Value: projectId},
			"user_id":    &dynamodbtypes.AttributeValueMemberS{Value: userId},
		},
	})

	return err
}

// GetProjectUser gets a project user by email or phone
func (d *DynamoDBDriver) GetProjectUser(ctx context.Context, projectId, email, phone string) (*types.DefaultDocumentStructure, error) {
	tableName := d.TablePrefix + "_users"

	var queryInput *dynamodb.QueryInput

	if email != "" {
		queryInput = &dynamodb.QueryInput{
			TableName:              aws.String(tableName),
			IndexName:              aws.String("EmailIndex"),
			KeyConditionExpression: aws.String("project_id = :project_id AND email = :email"),
			ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
				":project_id": &dynamodbtypes.AttributeValueMemberS{Value: projectId},
				":email":      &dynamodbtypes.AttributeValueMemberS{Value: email},
			},
			Limit: aws.Int32(1),
		}
	} else if phone != "" {
		queryInput = &dynamodb.QueryInput{
			TableName:              aws.String(tableName),
			IndexName:              aws.String("PhoneIndex"),
			KeyConditionExpression: aws.String("project_id = :project_id AND phone = :phone"),
			ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
				":project_id": &dynamodbtypes.AttributeValueMemberS{Value: projectId},
				":phone":      &dynamodbtypes.AttributeValueMemberS{Value: phone},
			},
			Limit: aws.Int32(1),
		}
	} else {
		return nil, fmt.Errorf("either email or phone must be provided")
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return nil, err
	}

	if len(result.Items) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	// Parse the result
	var user types.DefaultDocumentStructure
	if userDataAttr, ok := result.Items[0]["user_data"]; ok {
		if s, ok := userDataAttr.(*dynamodbtypes.AttributeValueMemberS); ok {
			err = json.Unmarshal([]byte(s.Value), &user)
			if err != nil {
				return nil, err
			}
		}
	}

	return &user, nil
}

// GetLoggedInProjectUser gets a logged-in project user by user ID
func (d *DynamoDBDriver) GetLoggedInProjectUser(ctx context.Context, projectId, userId string) (*types.DefaultDocumentStructure, error) {
	tableName := d.TablePrefix + "_users"

	input := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]dynamodbtypes.AttributeValue{
			"project_id": &dynamodbtypes.AttributeValueMemberS{Value: projectId},
			"user_id":    &dynamodbtypes.AttributeValueMemberS{Value: userId},
		},
	}

	result, err := d.Client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, fmt.Errorf("user not found")
	}

	// Parse the result
	var user types.DefaultDocumentStructure
	if userDataAttr, ok := result.Item["user_data"]; ok {
		if s, ok := userDataAttr.(*dynamodbtypes.AttributeValueMemberS); ok {
			err = json.Unmarshal([]byte(s.Value), &user)
			if err != nil {
				return nil, err
			}
		}
	}

	return &user, nil
}

// GetProjectUsers gets project users by their IDs
func (d *DynamoDBDriver) GetProjectUsers(ctx context.Context, projectId string, userIds []string) ([]*types.DefaultDocumentStructure, error) {
	tableName := d.TablePrefix + "_users"
	var users []*types.DefaultDocumentStructure

	for _, userId := range userIds {
		input := &dynamodb.GetItemInput{
			TableName: aws.String(tableName),
			Key: map[string]dynamodbtypes.AttributeValue{
				"project_id": &dynamodbtypes.AttributeValueMemberS{Value: projectId},
				"user_id":    &dynamodbtypes.AttributeValueMemberS{Value: userId},
			},
		}

		result, err := d.Client.GetItem(ctx, input)
		if err != nil {
			continue // Skip errors, continue with other users
		}

		if result.Item != nil {
			var user types.DefaultDocumentStructure
			if userDataAttr, ok := result.Item["user_data"]; ok {
				if s, ok := userDataAttr.(*dynamodbtypes.AttributeValueMemberS); ok {
					err = json.Unmarshal([]byte(s.Value), &user)
					if err == nil {
						users = append(users, &user)
					}
				}
			}
		}
	}

	return users, nil
}

// GetAllRelationDocumentsOfSingleDocument gets all relation documents for a single document
func (d *DynamoDBDriver) GetAllRelationDocumentsOfSingleDocument(ctx context.Context, param *models.CommonSystemParams, documentID string) ([]*types.DefaultDocumentStructure, error) {
	tableName := d.TablePrefix + "_relations"
	var relations []*types.DefaultDocumentStructure

	// Query relations where this document is the source (from_id)
	queryInput1 := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("FromIdIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND from_id = :from_id"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":from_id":    &dynamodbtypes.AttributeValueMemberS{Value: documentID},
		},
	}

	result1, err := d.Client.Query(ctx, queryInput1)
	if err != nil {
		return nil, err
	}

	// Parse from_id relations
	for _, item := range result1.Items {
		if relationDataAttr, ok := item["relation_data"]; ok {
			if s, ok := relationDataAttr.(*dynamodbtypes.AttributeValueMemberS); ok {
				var relation types.DefaultDocumentStructure
				err = json.Unmarshal([]byte(s.Value), &relation)
				if err == nil {
					relations = append(relations, &relation)
				}
			}
		}
	}

	// Query relations where this document is the target (to_id)
	queryInput2 := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("ToIdIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND to_id = :to_id"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":to_id":      &dynamodbtypes.AttributeValueMemberS{Value: documentID},
		},
	}

	result2, err := d.Client.Query(ctx, queryInput2)
	if err != nil {
		return nil, err
	}

	// Parse to_id relations
	for _, item := range result2.Items {
		if relationDataAttr, ok := item["relation_data"]; ok {
			if s, ok := relationDataAttr.(*dynamodbtypes.AttributeValueMemberS); ok {
				var relation types.DefaultDocumentStructure
				err = json.Unmarshal([]byte(s.Value), &relation)
				if err == nil {
					relations = append(relations, &relation)
				}
			}
		}
	}

	return relations, nil
}

// AddTeamMetaInfo adds team meta info to a document
func (d *DynamoDBDriver) AddTeamMetaInfo(ctx context.Context, param *models.CommonSystemParams, teamInfo *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	// Store team info as a regular document
	result, err := d.AddDocumentToProject(ctx, param, teamInfo)
	if err != nil {
		return nil, err
	}

	// AddDocumentToProject returns interface{}, we need to assert it back to our type
	if doc, ok := result.(*types.DefaultDocumentStructure); ok {
		return doc, nil
	}

	return teamInfo, nil
}

// RelationshipDataLoader loads relationship data
func (d *DynamoDBDriver) RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, ids []string) ([]*types.DefaultDocumentStructure, error) {
	tableName := d.TablePrefix + "_relations"
	var relations []*types.DefaultDocumentStructure

	for _, id := range ids {
		input := &dynamodb.GetItemInput{
			TableName: aws.String(tableName),
			Key: map[string]dynamodbtypes.AttributeValue{
				"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
				"relation_id": &dynamodbtypes.AttributeValueMemberS{Value: id},
			},
		}

		result, err := d.Client.GetItem(ctx, input)
		if err != nil {
			continue // Skip errors
		}

		if result.Item != nil {
			var relation types.DefaultDocumentStructure
			if relationDataAttr, ok := result.Item["relation_data"]; ok {
				if s, ok := relationDataAttr.(*dynamodbtypes.AttributeValueMemberS); ok {
					err = json.Unmarshal([]byte(s.Value), &relation)
					if err == nil {
						relations = append(relations, &relation)
					}
				}
			}
		}
	}

	return relations, nil
}

// RelationshipDataLoaderBytes loads relationship data as bytes
func (d *DynamoDBDriver) RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, ids []string) ([][]byte, error) {
	tableName := d.TablePrefix + "_relations"
	var relationBytes [][]byte

	for _, id := range ids {
		input := &dynamodb.GetItemInput{
			TableName: aws.String(tableName),
			Key: map[string]dynamodbtypes.AttributeValue{
				"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
				"relation_id": &dynamodbtypes.AttributeValueMemberS{Value: id},
			},
		}

		result, err := d.Client.GetItem(ctx, input)
		if err != nil {
			continue // Skip errors
		}

		if result.Item != nil {
			if relationDataAttr, ok := result.Item["relation_data"]; ok {
				if s, ok := relationDataAttr.(*dynamodbtypes.AttributeValueMemberS); ok {
					relationBytes = append(relationBytes, []byte(s.Value))
				}
			}
		}
	}

	return relationBytes, nil
}

// CountDocOfProject counts documents in a project
func (d *DynamoDBDriver) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (int, error) {
	tableName := d.TablePrefix + "_documents"

	// Use Query with Count
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("ModelIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND model_name = :model_name"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":model_name": &dynamodbtypes.AttributeValueMemberS{Value: param.Model.Name},
		},
		Select: dynamodbtypes.SelectCount,
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return 0, err
	}

	return int(result.Count), nil
}

// CountDocOfProjectBytes counts documents in a project (bytes version)
func (d *DynamoDBDriver) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) (int, error) {
	return d.CountDocOfProject(ctx, param)
}

// CountMultiDocumentOfProject counts multiple documents with filters
func (d *DynamoDBDriver) CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, condition map[string]interface{}) (int, error) {
	// For simplicity, return total count (DynamoDB filtering would require more complex implementation)
	return d.CountDocOfProject(ctx, param)
}

// AggregateDocOfProject performs aggregation on documents
func (d *DynamoDBDriver) AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams, pipeline interface{}) (interface{}, error) {
	// DynamoDB doesn't support complex aggregation pipelines like MongoDB
	// This would need custom implementation based on specific aggregation needs
	return nil, fmt.Errorf("aggregation not yet implemented for DynamoDB")
}

// AggregateDocOfProjectBytes performs aggregation on documents (bytes version)
func (d *DynamoDBDriver) AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams, pipeline interface{}) ([]byte, error) {
	result, err := d.AggregateDocOfProject(ctx, param, pipeline)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}

// DropField drops a field from all documents in a collection
func (d *DynamoDBDriver) DropField(ctx context.Context, param *models.CommonSystemParams, fieldName string) error {
	tableName := d.TablePrefix + "_documents"

	// Query all documents in this model
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("ModelIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND model_name = :model_name"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":model_name": &dynamodbtypes.AttributeValueMemberS{Value: param.Model.Name},
		},
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return err
	}

	// Update each document to remove the field
	for _, item := range result.Items {
		if docId, ok := item["document_id"]; ok {
			if docDataAttr, ok := item["document_data"]; ok {
				if s, ok := docId.(*dynamodbtypes.AttributeValueMemberS); ok {
					if dataS, ok := docDataAttr.(*dynamodbtypes.AttributeValueMemberS); ok {
						// Parse document data
						var doc types.DefaultDocumentStructure
						err = json.Unmarshal([]byte(dataS.Value), &doc)
						if err != nil {
							continue
						}

						// Remove the field
						if doc.Data != nil {
							delete(doc.Data, fieldName)
						}

						// Save back
						updatedDataJSON, err := json.Marshal(doc)
						if err != nil {
							continue
						}

						_, err = d.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
							TableName: aws.String(tableName),
							Key: map[string]dynamodbtypes.AttributeValue{
								"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
								"document_id": &dynamodbtypes.AttributeValueMemberS{Value: s.Value},
							},
							UpdateExpression: aws.String("SET document_data = :document_data, updated_at = :updated_at"),
							ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
								":document_data": &dynamodbtypes.AttributeValueMemberS{Value: string(updatedDataJSON)},
								":updated_at":    &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
							},
						})
						if err != nil {
							continue
						}
					}
				}
			}
		}
	}

	return nil
}

// RenameField renames a field in all documents in a collection
func (d *DynamoDBDriver) RenameField(ctx context.Context, param *models.CommonSystemParams, oldFieldName, newFieldName string) error {
	tableName := d.TablePrefix + "_documents"

	// Query all documents in this model
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("ModelIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND model_name = :model_name"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":model_name": &dynamodbtypes.AttributeValueMemberS{Value: param.Model.Name},
		},
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return err
	}

	// Update each document to rename the field
	for _, item := range result.Items {
		if docId, ok := item["document_id"]; ok {
			if docDataAttr, ok := item["document_data"]; ok {
				if s, ok := docId.(*dynamodbtypes.AttributeValueMemberS); ok {
					if dataS, ok := docDataAttr.(*dynamodbtypes.AttributeValueMemberS); ok {
						// Parse document data
						var doc types.DefaultDocumentStructure
						err = json.Unmarshal([]byte(dataS.Value), &doc)
						if err != nil {
							continue
						}

						// Rename the field
						if doc.Data != nil {
							if value, exists := doc.Data[oldFieldName]; exists {
								doc.Data[newFieldName] = value
								delete(doc.Data, oldFieldName)
							}
						}

						// Save back
						updatedDataJSON, err := json.Marshal(doc)
						if err != nil {
							continue
						}

						_, err = d.Client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
							TableName: aws.String(tableName),
							Key: map[string]dynamodbtypes.AttributeValue{
								"project_id":  &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
								"document_id": &dynamodbtypes.AttributeValueMemberS{Value: s.Value},
							},
							UpdateExpression: aws.String("SET document_data = :document_data, updated_at = :updated_at"),
							ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
								":document_data": &dynamodbtypes.AttributeValueMemberS{Value: string(updatedDataJSON)},
								":updated_at":    &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
							},
						})
						if err != nil {
							continue
						}
					}
				}
			}
		}
	}

	return nil
}

// DeleteMediaFile deletes a media file document
func (d *DynamoDBDriver) DeleteMediaFile(ctx context.Context, param *models.CommonSystemParams) error {
	return d.DeleteDocumentFromProject(ctx, param)
}

// DuplicateModel duplicates a model by copying all its documents
func (d *DynamoDBDriver) DuplicateModel(ctx context.Context, param *models.CommonSystemParams, newModelName string) error {
	tableName := d.TablePrefix + "_documents"

	// Query all documents in the source model
	queryInput := &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("ModelIndex"),
		KeyConditionExpression: aws.String("project_id = :project_id AND model_name = :model_name"),
		ExpressionAttributeValues: map[string]dynamodbtypes.AttributeValue{
			":project_id": &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
			":model_name": &dynamodbtypes.AttributeValueMemberS{Value: param.Model.Name},
		},
	}

	result, err := d.Client.Query(ctx, queryInput)
	if err != nil {
		return err
	}

	// Copy each document to the new model
	for _, item := range result.Items {
		if docDataAttr, ok := item["document_data"]; ok {
			if dataS, ok := docDataAttr.(*dynamodbtypes.AttributeValueMemberS); ok {
				// Parse original document
				var doc types.DefaultDocumentStructure
				err = json.Unmarshal([]byte(dataS.Value), &doc)
				if err != nil {
					continue
				}

				// Create new document with new ID and model name
				doc.ID = uuid.New().String()

				documentDataJSON, err := json.Marshal(doc)
				if err != nil {
					continue
				}

				newItem := map[string]dynamodbtypes.AttributeValue{
					"project_id":    &dynamodbtypes.AttributeValueMemberS{Value: param.ProjectID},
					"document_id":   &dynamodbtypes.AttributeValueMemberS{Value: doc.ID},
					"model_name":    &dynamodbtypes.AttributeValueMemberS{Value: newModelName},
					"document_data": &dynamodbtypes.AttributeValueMemberS{Value: string(documentDataJSON)},
					"created_at":    &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
					"updated_at":    &dynamodbtypes.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
				}

				_, err = d.Client.PutItem(ctx, &dynamodb.PutItemInput{
					TableName: aws.String(tableName),
					Item:      newItem,
				})
				if err != nil {
					continue
				}
			}
		}
	}

	return nil
}

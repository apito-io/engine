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

// AddRelationFields creates a relation field (has one or has many) between models
func (c *CouchbaseDriver) AddRelationFields(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error {
	// In Couchbase, relations are handled through document references
	// This is a schema-level operation that creates the relation structure
	return nil
}

// DeleteRelationDocuments drops pivot tables, relation keys, or collection tables and all documents within them
func (c *CouchbaseDriver) DeleteRelationDocuments(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error {
	// Delete all relations between the specified models
	query := fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"project_relation\" AND project_id = \"%s\" AND from_model = \"%s\" AND to_model = \"%s\"",
		c.Bucket.Name(), projectId, from.Model, to.Model)

	_, err := c.Cluster.Query(query, nil)
	return err
}

// GetRelationDocument retrieves a relation document by ID
func (c *CouchbaseDriver) GetRelationDocument(ctx context.Context, param *models.ConnectDisconnectParam) (*models.EdgeRelation, error) {
	relationKey := fmt.Sprintf("relation::%s::%s::%s", param.DocCollectionName, param.CurrentActionID, param.ForwardConnectionID)

	result, err := c.Collection.Get(relationKey, nil)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := result.Content(&data); err != nil {
		return nil, err
	}

	if relationDataStr, ok := data["relation_data"].(string); ok {
		var relation models.EdgeRelation
		if err := json.Unmarshal([]byte(relationDataStr), &relation); err != nil {
			return nil, err
		}
		return &relation, nil
	}

	return nil, fmt.Errorf("relation data not found")
}

// CreateRelation creates a relation in the project
func (c *CouchbaseDriver) CreateRelation(ctx context.Context, projectId string, relation *models.EdgeRelation) error {
	if relation.Key == "" {
		relation.Key = uuid.New().String()
	}

	relation.CreatedAt = time.Now().Format(time.RFC3339)

	relationDataJSON, err := json.Marshal(relation)
	if err != nil {
		return err
	}

	relationKey := fmt.Sprintf("relation::%s::%s::%s", projectId, relation.FromID, relation.ToID)
	relationDoc := map[string]interface{}{
		"doc_type":      "project_relation",
		"project_id":    projectId,
		"from_id":       relation.FromID,
		"to_id":         relation.ToID,
		"from_model":    relation.From,
		"to_model":      relation.To,
		"relation_type": relation.Relation,
		"relation_data": string(relationDataJSON),
		"created_at":    time.Now().Format(time.RFC3339),
	}

	_, err = c.Collection.Upsert(relationKey, relationDoc, nil)
	return err
}

// DeleteRelation deletes a relation in the project
func (c *CouchbaseDriver) DeleteRelation(ctx context.Context, param *models.ConnectDisconnectParam, id string) error {
	relationKey := fmt.Sprintf("relation::%s::%s::%s", param.DocCollectionName, param.CurrentActionID, id)
	_, err := c.Collection.Remove(relationKey, nil)
	return err
}

// NewInsertableRelations retrieves new insertable relations in the project
func (c *CouchbaseDriver) NewInsertableRelations(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	// Get existing relations for this document
	query := fmt.Sprintf("SELECT to_id FROM `%s` WHERE doc_type = \"project_relation\" AND project_id = \"%s\" AND from_id = \"%s\"",
		c.Bucket.Name(), param.DocCollectionName, param.CurrentActionID)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return nil, err
	}
	defer results.Close()

	existingRelations := make(map[string]bool)
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if toID, ok := row["to_id"].(string); ok {
			existingRelations[toID] = true
		}
	}

	// Return action IDs that are not already related
	var insertableRelations []string
	for _, actionID := range param.ActionIDs {
		if !existingRelations[actionID] {
			insertableRelations = append(insertableRelations, actionID)
		}
	}

	return insertableRelations, nil
}

// CheckOneToOneRelationExists checks if a one-to-one relation exists in the project
func (c *CouchbaseDriver) CheckOneToOneRelationExists(ctx context.Context, param *models.ConnectDisconnectParam) (bool, error) {
	query := fmt.Sprintf("SELECT COUNT(*) as count FROM `%s` WHERE doc_type = \"project_relation\" AND project_id = \"%s\" AND from_id = \"%s\" AND relation_type = \"one_to_one\"",
		c.Bucket.Name(), param.DocCollectionName, param.CurrentActionID)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return false, err
	}
	defer results.Close()

	if !results.Next() {
		return false, nil
	}

	var row map[string]interface{}
	if err := results.Row(&row); err != nil {
		return false, err
	}

	count, _ := row["count"].(float64)
	return count > 0, nil
}

// GetRelationIds retrieves the IDs of every document related to a document
func (c *CouchbaseDriver) GetRelationIds(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	query := fmt.Sprintf("SELECT to_id FROM `%s` WHERE doc_type = \"project_relation\" AND project_id = \"%s\" AND from_id = \"%s\"",
		c.Bucket.Name(), param.DocCollectionName, param.CurrentActionID)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var relationIds []string
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if toID, ok := row["to_id"].(string); ok {
			relationIds = append(relationIds, toID)
		}
	}

	return relationIds, nil
}

// ConnectBuilder connects a builder to the project
func (c *CouchbaseDriver) ConnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	builderKey := fmt.Sprintf("builder::%s::%s", param.ProjectID, param.UserID)
	builderDoc := map[string]interface{}{
		"doc_type":   "project_builder",
		"project_id": param.ProjectID,
		"user_id":    param.UserID,
		"connected":  true,
		"created_at": time.Now().Format(time.RFC3339),
	}

	_, err := c.Collection.Upsert(builderKey, builderDoc, nil)
	return err
}

// DisconnectBuilder disconnects a builder from the project
func (c *CouchbaseDriver) DisconnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	builderKey := fmt.Sprintf("builder::%s::%s", param.ProjectID, param.UserID)
	_, err := c.Collection.Remove(builderKey, nil)
	return err
}

// GetProjectUser retrieves a user profile by phone, email, and project ID
func (c *CouchbaseDriver) GetProjectUser(ctx context.Context, phone, email, projectId string) (*types.DefaultDocumentStructure, error) {
	var query string
	var queryParam interface{}

	if email != "" {
		query = fmt.Sprintf("SELECT document_data FROM `%s` WHERE doc_type = \"project_user\" AND project_id = \"%s\" AND email = $1 LIMIT 1",
			c.Bucket.Name(), projectId)
		queryParam = email
	} else if phone != "" {
		query = fmt.Sprintf("SELECT document_data FROM `%s` WHERE doc_type = \"project_user\" AND project_id = \"%s\" AND phone = $1 LIMIT 1",
			c.Bucket.Name(), projectId)
		queryParam = phone
	} else {
		return nil, fmt.Errorf("email or phone must be provided")
	}

	results, err := c.Cluster.Query(query, &gocb.QueryOptions{
		PositionalParameters: []interface{}{queryParam},
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

	if userDataStr, ok := row["document_data"].(string); ok {
		var user types.DefaultDocumentStructure
		if err := json.Unmarshal([]byte(userDataStr), &user); err != nil {
			return nil, err
		}
		return &user, nil
	}

	return nil, fmt.Errorf("user data not found")
}

// GetLoggedInProjectUser retrieves the logged-in user profile for the project
func (c *CouchbaseDriver) GetLoggedInProjectUser(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	userKey := fmt.Sprintf("user::%s::%s", param.ProjectID, param.UserID)

	result, err := c.Collection.Get(userKey, nil)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := result.Content(&data); err != nil {
		return nil, err
	}

	if userDataStr, ok := data["document_data"].(string); ok {
		var user types.DefaultDocumentStructure
		if err := json.Unmarshal([]byte(userDataStr), &user); err != nil {
			return nil, err
		}
		return &user, nil
	}

	return nil, fmt.Errorf("user data not found")
}

// GetProjectUsers retrieves metadata for multiple users in the project
func (c *CouchbaseDriver) GetProjectUsers(ctx context.Context, projectId string, keys []string) (map[string]*types.DefaultDocumentStructure, error) {
	result := make(map[string]*types.DefaultDocumentStructure)

	for _, key := range keys {
		userKey := fmt.Sprintf("user::%s::%s", projectId, key)

		docResult, err := c.Collection.Get(userKey, nil)
		if err != nil {
			continue // Skip missing users
		}

		var data map[string]interface{}
		if err := docResult.Content(&data); err != nil {
			continue
		}

		if userDataStr, ok := data["document_data"].(string); ok {
			var user types.DefaultDocumentStructure
			if err := json.Unmarshal([]byte(userDataStr), &user); err == nil {
				result[key] = &user
			}
		}
	}

	return result, nil
}

// GetAllRelationDocumentsOfSingleDocument retrieves all relation data of a single document by ID
func (c *CouchbaseDriver) GetAllRelationDocumentsOfSingleDocument(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error) {
	query := fmt.Sprintf("SELECT relation_data FROM `%s` WHERE doc_type = \"project_relation\" AND project_id = \"%s\" AND from_id = \"%s\"",
		c.Bucket.Name(), arg.ProjectID, from)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var relations []interface{}
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if relationDataStr, ok := row["relation_data"].(string); ok {
			var relation interface{}
			if err := json.Unmarshal([]byte(relationDataStr), &relation); err == nil {
				relations = append(relations, relation)
			}
		}
	}

	return relations, nil
}

// AddTeamMetaInfo adds metadata information for a team in the project
func (c *CouchbaseDriver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	for _, doc := range docs {
		metadataJSON, err := json.Marshal(doc)
		if err != nil {
			return nil, err
		}

		metaKey := fmt.Sprintf("team_meta::%s", doc.ID)
		metaDoc := map[string]interface{}{
			"doc_type":   "team_metadata",
			"user_id":    doc.ID,
			"metadata":   string(metadataJSON),
			"created_at": time.Now().Format(time.RFC3339),
		}

		_, err = c.Collection.Upsert(metaKey, metaDoc, nil)
		if err != nil {
			return nil, err
		}
	}

	return docs, nil
}

// RelationshipDataLoader loads relationship data for the project
func (c *CouchbaseDriver) RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) (interface{}, error) {
	query := fmt.Sprintf("SELECT relation_data FROM `%s` WHERE doc_type = \"project_relation\" AND project_id = \"%s\" AND from_id = \"%s\"",
		c.Bucket.Name(), param.ProjectID, param.DocumentID)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var relations []interface{}
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if relationDataStr, ok := row["relation_data"].(string); ok {
			var relation interface{}
			if err := json.Unmarshal([]byte(relationDataStr), &relation); err == nil {
				relations = append(relations, relation)
			}
		}
	}

	return relations, nil
}

// RelationshipDataLoaderBytes loads relationship data for the project and returns it as bytes
func (c *CouchbaseDriver) RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) ([]byte, error) {
	data, err := c.RelationshipDataLoader(ctx, param, connection)
	if err != nil {
		return nil, err
	}

	return json.Marshal(data)
}

// CountDocOfProject counts the documents in the project
func (c *CouchbaseDriver) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	query := fmt.Sprintf("SELECT COUNT(*) as total FROM `%s` WHERE doc_type = \"project_document_%s\" AND project_id = \"%s\"",
		c.Bucket.Name(), param.Model.Name, param.ProjectID)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return nil, err
	}
	defer results.Close()

	if !results.Next() {
		return map[string]interface{}{"total": 0}, nil
	}

	var row map[string]interface{}
	if err := results.Row(&row); err != nil {
		return nil, err
	}

	return row, nil
}

// CountDocOfProjectBytes counts the documents in the project and returns the result as bytes
func (c *CouchbaseDriver) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	data, err := c.CountDocOfProject(ctx, param)
	if err != nil {
		return nil, err
	}

	return json.Marshal(data)
}

// CountMultiDocumentOfProject counts multiple documents in the project
func (c *CouchbaseDriver) CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, previewModel bool) (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) as count FROM `%s` WHERE doc_type = \"project_document_%s\" AND project_id = \"%s\"",
		c.Bucket.Name(), param.Model.Name, param.ProjectID)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return 0, err
	}
	defer results.Close()

	if !results.Next() {
		return 0, nil
	}

	var row map[string]interface{}
	if err := results.Row(&row); err != nil {
		return 0, err
	}

	count, _ := row["count"].(float64)
	return int(count), nil
}

// AggregateDocOfProject aggregates the documents in the project
func (c *CouchbaseDriver) AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	query := fmt.Sprintf("SELECT COUNT(*) as count, MIN(created_at) as earliest, MAX(updated_at) as latest FROM `%s` WHERE doc_type = \"project_document_%s\" AND project_id = \"%s\"",
		c.Bucket.Name(), param.Model.Name, param.ProjectID)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return nil, err
	}
	defer results.Close()

	if !results.Next() {
		return map[string]interface{}{
			"count":    0,
			"earliest": nil,
			"latest":   nil,
		}, nil
	}

	var row map[string]interface{}
	if err := results.Row(&row); err != nil {
		return nil, err
	}

	return row, nil
}

// AggregateDocOfProjectBytes aggregates the documents in the project and returns the result as bytes
func (c *CouchbaseDriver) AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	data, err := c.AggregateDocOfProject(ctx, param)
	if err != nil {
		return nil, err
	}

	return json.Marshal(data)
}

// DropField drops/deletes a field and its data from the project
func (c *CouchbaseDriver) DropField(ctx context.Context, param *models.CommonSystemParams) error {
	// In Couchbase, since we store documents as JSON, we need to update all documents
	// to remove the specified field

	// Get all documents for this model
	query := fmt.Sprintf("SELECT META().id, document_data FROM `%s` WHERE doc_type = \"project_document_%s\" AND project_id = \"%s\"",
		c.Bucket.Name(), param.Model.Name, param.ProjectID)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return err
	}
	defer results.Close()

	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		docID, _ := row["id"].(string)
		documentDataStr, _ := row["document_data"].(string)

		var document map[string]interface{}
		if err := json.Unmarshal([]byte(documentDataStr), &document); err != nil {
			continue
		}

		// Remove the field
		if param.FieldInfo != nil {
			delete(document, param.FieldInfo.Identifier)
		}

		// Update the document
		updatedJSON, err := json.Marshal(document)
		if err != nil {
			continue
		}

		updateQuery := fmt.Sprintf("UPDATE `%s` SET document_data = $1 WHERE META().id = $2")
		c.Cluster.Query(updateQuery, &gocb.QueryOptions{
			PositionalParameters: []interface{}{string(updatedJSON), docID},
		})
	}

	return nil
}

// RenameField renames a field in a model along with its data key
func (c *CouchbaseDriver) RenameField(ctx context.Context, oldFieldName string, repeatedFieldGroup string, param *models.CommonSystemParams) error {
	// In Couchbase, since we store documents as JSON, we need to update all documents
	// to rename the specified field

	// Get all documents for this model
	query := fmt.Sprintf("SELECT META().id, document_data FROM `%s` WHERE doc_type = \"project_document_%s\" AND project_id = \"%s\"",
		c.Bucket.Name(), param.Model.Name, param.ProjectID)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return err
	}
	defer results.Close()

	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		docID, _ := row["id"].(string)
		documentDataStr, _ := row["document_data"].(string)

		var document map[string]interface{}
		if err := json.Unmarshal([]byte(documentDataStr), &document); err != nil {
			continue
		}

		// Rename the field
		if param.FieldInfo != nil && oldFieldName != "" {
			if value, exists := document[oldFieldName]; exists {
				document[param.FieldInfo.Identifier] = value
				delete(document, oldFieldName)
			}
		}

		// Update the document
		updatedJSON, err := json.Marshal(document)
		if err != nil {
			continue
		}

		updateQuery := fmt.Sprintf("UPDATE `%s` SET document_data = $1 WHERE META().id = $2")
		c.Cluster.Query(updateQuery, &gocb.QueryOptions{
			PositionalParameters: []interface{}{string(updatedJSON), docID},
		})
	}

	return nil
}

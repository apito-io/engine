package couchbase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/google/uuid"
)

// CheckCollectionExists checks if a collection exists in the project
func (c *CouchbaseDriver) CheckCollectionExists(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) (bool, error) {
	var docType string
	if isRelationCollection {
		docType = "project_relation"
	} else {
		docType = fmt.Sprintf("project_document_%s", param.Model.Name)
	}

	// Check if any documents with this doc_type exist
	query := fmt.Sprintf("SELECT COUNT(*) as count FROM `%s` WHERE doc_type = \"%s\" AND project_id = \"%s\" LIMIT 1",
		c.Bucket.Name(), docType, param.ProjectID)

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
	return count >= 0, nil // Collection exists if we can query it, regardless of document count
}

// AddCollection adds a new collection to the project
func (c *CouchbaseDriver) AddCollection(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) error {
	var docType string
	if isRelationCollection {
		docType = "project_relation"
	} else {
		docType = fmt.Sprintf("project_document_%s", param.Model.Name)
	}

	// Create a metadata document to indicate collection existence
	metaDocKey := fmt.Sprintf("collection_meta::%s::%s", param.ProjectID, docType)
	metaDoc := map[string]interface{}{
		"doc_type":               "collection_metadata",
		"project_id":             param.ProjectID,
		"model_name":             param.Model.Name,
		"created_at":             time.Now().Format(time.RFC3339),
		"is_relation_collection": isRelationCollection,
	}

	_, err := c.Collection.Upsert(metaDocKey, metaDoc, nil)
	return err
}

// AddModel adds a new model to the project
func (c *CouchbaseDriver) AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error) {
	// Create model metadata document
	modelDocKey := fmt.Sprintf("model_meta::%s::%s", project.ID, model.Name)

	modelDataJSON, err := json.Marshal(model)
	if err != nil {
		return nil, err
	}

	modelDoc := map[string]interface{}{
		"doc_type":   "project_model",
		"project_id": project.ID,
		"model_name": model.Name,
		"model_data": string(modelDataJSON),
		"created_at": time.Now().Format(time.RFC3339),
	}

	_, err = c.Collection.Upsert(modelDocKey, modelDoc, nil)
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
func (c *CouchbaseDriver) AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parent_field string) (*models.ModelType, error) {
	// In Couchbase, fields are managed in the model metadata
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

	// Update model metadata document
	modelDocKey := fmt.Sprintf("model_meta::%s::%s", param.ProjectID, param.Model.Name)
	modelDataJSON, err := json.Marshal(param.Model)
	if err != nil {
		return nil, err
	}

	modelDoc := map[string]interface{}{
		"doc_type":   "project_model",
		"project_id": param.ProjectID,
		"model_name": param.Model.Name,
		"model_data": string(modelDataJSON),
		"updated_at": time.Now().Format(time.RFC3339),
	}

	_, err = c.Collection.Upsert(modelDocKey, modelDoc, nil)
	return param.Model, err
}

// RenameModel renames a model in the project
func (c *CouchbaseDriver) RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error {
	oldModelKey := fmt.Sprintf("model_meta::%s::%s", project.ID, modelName)
	newModelKey := fmt.Sprintf("model_meta::%s::%s", project.ID, newName)

	// Get the old model document
	result, err := c.Collection.Get(oldModelKey, nil)
	if err != nil {
		return err
	}

	var modelDoc map[string]interface{}
	if err := result.Content(&modelDoc); err != nil {
		return err
	}

	// Update the model name in the document
	modelDoc["model_name"] = newName
	modelDoc["updated_at"] = time.Now().Format(time.RFC3339)

	// Create new document and delete old one
	_, err = c.Collection.Upsert(newModelKey, modelDoc, nil)
	if err != nil {
		return err
	}

	// Update all documents belonging to this model
	query := fmt.Sprintf("UPDATE `%s` SET model_name = \"%s\" WHERE doc_type = \"project_document_%s\" AND project_id = \"%s\"",
		c.Bucket.Name(), newName, modelName, project.ID)

	_, err = c.Cluster.Query(query, nil)
	if err != nil {
		return err
	}

	// Delete old model metadata
	_, err = c.Collection.Remove(oldModelKey, nil)
	return err
}

// ConvertModel converts a model in the project
func (c *CouchbaseDriver) ConvertModel(ctx context.Context, project *models.Project, modelName string) error {
	// Model conversion logic - implementation specific to business needs
	// For now, return nil as this is a complex operation
	return nil
}

// DropModel drops a model from the project
func (c *CouchbaseDriver) DropModel(ctx context.Context, project *models.Project, modelName string) error {
	// Delete all documents belonging to this model
	query := fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"project_document_%s\" AND project_id = \"%s\"",
		c.Bucket.Name(), modelName, project.ID)

	_, err := c.Cluster.Query(query, nil)
	if err != nil {
		return err
	}

	// Delete model metadata
	modelDocKey := fmt.Sprintf("model_meta::%s::%s", project.ID, modelName)
	_, err = c.Collection.Remove(modelDocKey, nil)

	return err
}

// CreateIndex creates an index for a model in the project
func (c *CouchbaseDriver) CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parent_field string) error {
	indexName := fmt.Sprintf("idx_%s_%s_%s", param.ProjectID, param.Model.Name, fieldName)

	// Create index on document_data field using N1QL
	query := fmt.Sprintf("CREATE INDEX `%s` ON `%s`(document_data) WHERE doc_type = \"project_document_%s\" AND project_id = \"%s\"",
		indexName, c.Bucket.Name(), param.Model.Name, param.ProjectID)

	_, err := c.Cluster.Query(query, nil)
	return err
}

// DropIndex drops an index from a model in the project
func (c *CouchbaseDriver) DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error {
	query := fmt.Sprintf("DROP INDEX `%s` ON `%s`", indexName, c.Bucket.Name())
	_, err := c.Cluster.Query(query, nil)
	return err
}

// GetSingleProjectDocument retrieves a single project document by ID
func (c *CouchbaseDriver) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	docKey := fmt.Sprintf("doc::%s::%s::%s", param.ProjectID, param.Model.Name, param.DocumentID)

	result, err := c.Collection.Get(docKey, nil)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := result.Content(&data); err != nil {
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
func (c *CouchbaseDriver) GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	docKey := fmt.Sprintf("doc::%s::%s::%s", param.ProjectID, param.Model.Name, param.DocumentID)

	result, err := c.Collection.Get(docKey, nil)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := result.Content(&data); err != nil {
		return nil, err
	}

	if documentDataStr, ok := data["document_data"].(string); ok {
		return []byte(documentDataStr), nil
	}

	return nil, fmt.Errorf("document data not found")
}

// GetSingleProjectDocumentRevisions retrieves the revision history of a single project document by ID
func (c *CouchbaseDriver) GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*models.DocumentRevisionHistory, error) {
	query := fmt.Sprintf("SELECT revision_data FROM `%s` WHERE doc_type = \"project_revision\" AND project_id = \"%s\" AND document_id = \"%s\" ORDER BY created_at DESC",
		c.Bucket.Name(), param.ProjectID, param.DocumentID)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var revisions []*models.DocumentRevisionHistory
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if revisionDataStr, ok := row["revision_data"].(string); ok {
			var revision models.DocumentRevisionHistory
			if err := json.Unmarshal([]byte(revisionDataStr), &revision); err == nil {
				revisions = append(revisions, &revision)
			}
		}
	}

	return revisions, nil
}

// GetSingleRawDocumentFromProject retrieves a single raw document from the project
func (c *CouchbaseDriver) GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	docKey := fmt.Sprintf("doc::%s::%s::%s", param.ProjectID, param.Model.Name, param.DocumentID)

	result, err := c.Collection.Get(docKey, nil)
	if err != nil {
		return nil, err
	}

	var data map[string]interface{}
	if err := result.Content(&data); err != nil {
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
func (c *CouchbaseDriver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
	query := fmt.Sprintf("SELECT document_data FROM `%s` WHERE doc_type = \"project_document_%s\" AND project_id = \"%s\" LIMIT 100",
		c.Bucket.Name(), param.Model.Name, param.ProjectID)

	results, err := c.Cluster.Query(query, nil)
	if err != nil {
		return nil, err
	}
	defer results.Close()

	var documents []*types.DefaultDocumentStructure
	for results.Next() {
		var row map[string]interface{}
		if err := results.Row(&row); err != nil {
			continue
		}

		if documentDataStr, ok := row["document_data"].(string); ok {
			var document types.DefaultDocumentStructure
			if err := json.Unmarshal([]byte(documentDataStr), &document); err == nil {
				documents = append(documents, &document)
			}
		}
	}

	return documents, nil
}

// QueryMultiDocumentOfProjectBytes queries multiple documents in the project and returns the result as bytes
func (c *CouchbaseDriver) QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	documents, err := c.QueryMultiDocumentOfProject(ctx, param)
	if err != nil {
		return nil, err
	}

	return json.Marshal(documents)
}

// AddDocumentToProject adds a new document to the project
func (c *CouchbaseDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
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

	docKey := fmt.Sprintf("doc::%s::%s::%s", param.ProjectID, param.Model.Name, doc.ID)
	documentDoc := map[string]interface{}{
		"doc_type":      fmt.Sprintf("project_document_%s", param.Model.Name),
		"project_id":    param.ProjectID,
		"model_name":    param.Model.Name,
		"document_id":   doc.ID,
		"document_data": string(documentDataJSON),
		"created_at":    time.Now().Format(time.RFC3339),
		"updated_at":    time.Now().Format(time.RFC3339),
	}

	_, err = c.Collection.Upsert(docKey, documentDoc, nil)
	return doc, err
}

// UpdateDocumentOfProject updates a particular document in the project
func (c *CouchbaseDriver) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {
	// Initialize Meta field if nil
	if doc.Meta == nil {
		doc.Meta = &types.MetaField{}
	}

	doc.Meta.UpdatedAt = time.Now().Format(time.RFC3339)

	documentDataJSON, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	docKey := fmt.Sprintf("doc::%s::%s::%s", param.ProjectID, param.Model.Name, doc.ID)
	documentDoc := map[string]interface{}{
		"doc_type":      fmt.Sprintf("project_document_%s", param.Model.Name),
		"project_id":    param.ProjectID,
		"model_name":    param.Model.Name,
		"document_id":   doc.ID,
		"document_data": string(documentDataJSON),
		"updated_at":    time.Now().Format(time.RFC3339),
	}

	_, err = c.Collection.Upsert(docKey, documentDoc, nil)
	return err
}

// DeleteDocumentFromProject deletes a document from the project
func (c *CouchbaseDriver) DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	docKey := fmt.Sprintf("doc::%s::%s::%s", param.ProjectID, param.Model.Name, param.DocumentID)
	_, err := c.Collection.Remove(docKey, nil)
	return err
}

// DeleteDocumentsFromProject deletes multiple documents from the project
func (c *CouchbaseDriver) DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	for _, docID := range param.DocumentIDs {
		docKey := fmt.Sprintf("doc::%s::%s::%s", param.ProjectID, param.Model.Name, docID)
		_, err := c.Collection.Remove(docKey, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteDocumentRelation deletes all relations or data in pivot tables from the project
func (c *CouchbaseDriver) DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error {
	// Delete relations where this document is the source
	query1 := fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"project_relation\" AND project_id = \"%s\" AND from_id = \"%s\"",
		c.Bucket.Name(), param.ProjectID, param.DocumentID)

	_, err := c.Cluster.Query(query1, nil)
	if err != nil {
		return err
	}

	// Delete relations where this document is the target
	query2 := fmt.Sprintf("DELETE FROM `%s` WHERE doc_type = \"project_relation\" AND project_id = \"%s\" AND to_id = \"%s\"",
		c.Bucket.Name(), param.ProjectID, param.DocumentID)

	_, err = c.Cluster.Query(query2, nil)
	return err
}

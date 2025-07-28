package badger

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

// CheckCollectionExists checks if a collection exists in the project
func (b *BadgerDriver) CheckCollectionExists(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) (bool, error) {
	var prefix string
	if isRelationCollection {
		prefix = b.generateKey("project_rel", param.ProjectID)
	} else {
		prefix = b.generateKey("project_doc", param.ProjectID, param.Model.Name)
	}

	// Check if any keys exist with this prefix
	var exists bool
	err := b.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefixBytes := []byte(prefix)
		it.Seek(prefixBytes)
		exists = it.ValidForPrefix(prefixBytes)
		return nil
	})

	return exists, err
}

// AddCollection adds a new collection to the project
func (b *BadgerDriver) AddCollection(ctx context.Context, param *models.CommonSystemParams) error {
	// Initialize metadata if not exists
	if err := b.initializeMetadata(ctx, param.ProjectID); err != nil {
		return err
	}

	// Store model metadata
	modelKey := b.generateKey("project_model", param.ProjectID, param.Model.Name)

	modelData := map[string]interface{}{
		"model":      param.Model,
		"created_at": time.Now().Format(time.RFC3339),
	}

	modelJSON, err := json.Marshal(modelData)
	if err != nil {
		return err
	}

	return b.setValue(modelKey, modelJSON)
}

// AddModel adds a new model to the project
func (b *BadgerDriver) AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error) {
	// Initialize metadata if not exists
	if err := b.initializeMetadata(ctx, project.ID); err != nil {
		return nil, err
	}

	// Store model metadata
	modelKey := b.generateKey("project_model", project.ID, model.Name)
	modelData := map[string]interface{}{
		"model":      model,
		"created_at": time.Now().Format(time.RFC3339),
	}

	modelJSON, err := json.Marshal(modelData)
	if err != nil {
		return nil, err
	}

	if err := b.setValue(modelKey, modelJSON); err != nil {
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
func (b *BadgerDriver) AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parent_field string) (*models.ModelType, error) {
	modelKey := b.generateKey("project_model", param.ProjectID, param.Model.Name)

	// Get current model data
	modelJSON, err := b.getValue(modelKey)
	if err != nil {
		return nil, err
	}

	var modelData map[string]interface{}
	if err := json.Unmarshal(modelJSON, &modelData); err != nil {
		return nil, err
	}

	// Extract the model
	modelBytes, err := json.Marshal(modelData["model"])
	if err != nil {
		return nil, err
	}

	var model models.ModelType
	if err := json.Unmarshal(modelBytes, &model); err != nil {
		return nil, err
	}

	// Add the new field
	model.Fields = append(model.Fields, param.FieldInfo)

	// Update the model
	modelData["model"] = model
	modelData["updated_at"] = time.Now().Format(time.RFC3339)

	updatedModelJSON, err := json.Marshal(modelData)
	if err != nil {
		return nil, err
	}

	if err := b.setValue(modelKey, updatedModelJSON); err != nil {
		return nil, err
	}

	return &model, nil
}

// RenameModel renames a model in the project
func (b *BadgerDriver) RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error {
	return b.DB.Update(func(txn *badger.Txn) error {
		// Get the old model
		oldModelKey := b.generateKey("project_model", project.ID, modelName)
		item, err := txn.Get([]byte(oldModelKey))
		if err != nil {
			return err
		}

		var modelData []byte
		err = item.Value(func(val []byte) error {
			modelData = append([]byte(nil), val...)
			return nil
		})
		if err != nil {
			return err
		}

		// Update model name in the data
		var modelInfo map[string]interface{}
		if err := json.Unmarshal(modelData, &modelInfo); err != nil {
			return err
		}

		if modelObj, ok := modelInfo["model"].(map[string]interface{}); ok {
			modelObj["name"] = newName
		}
		modelInfo["updated_at"] = time.Now().Format(time.RFC3339)

		updatedModelData, err := json.Marshal(modelInfo)
		if err != nil {
			return err
		}

		// Save with new key
		newModelKey := b.generateKey("project_model", project.ID, newName)
		if err := txn.Set([]byte(newModelKey), updatedModelData); err != nil {
			return err
		}

		// Delete old key
		if err := txn.Delete([]byte(oldModelKey)); err != nil {
			return err
		}

		// Update all documents that use this model
		prefix := b.generateKey("project_doc", project.ID, modelName)
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		var documentsToUpdate []struct {
			oldKey string
			newKey string
			data   []byte
		}

		prefixBytes := []byte(prefix)
		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			item := it.Item()
			oldKey := string(item.Key())

			err := item.Value(func(val []byte) error {
				// Generate new key with updated model name
				_, projectID, parts := b.parseKey(oldKey)
				if len(parts) >= 2 {
					parts[0] = newName // Update model name in key
					newKey := b.generateKey("project_doc", projectID, parts...)
					documentsToUpdate = append(documentsToUpdate, struct {
						oldKey string
						newKey string
						data   []byte
					}{oldKey, newKey, append([]byte(nil), val...)})
				}
				return nil
			})

			if err != nil {
				return err
			}
		}

		// Apply document updates
		for _, update := range documentsToUpdate {
			if err := txn.Set([]byte(update.newKey), update.data); err != nil {
				return err
			}
			if err := txn.Delete([]byte(update.oldKey)); err != nil {
				return err
			}
		}

		return nil
	})
}

// ConvertModel converts a model in the project (no-op for BadgerDB)
func (b *BadgerDriver) ConvertModel(ctx context.Context, project *models.Project, modelName string) error {
	// BadgerDB doesn't require model conversion
	return nil
}

// DropModel drops a model from the project
func (b *BadgerDriver) DropModel(ctx context.Context, project *models.Project, modelName string) error {
	return b.DB.Update(func(txn *badger.Txn) error {
		// Delete model metadata
		modelKey := b.generateKey("project_model", project.ID, modelName)
		if err := txn.Delete([]byte(modelKey)); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		// Delete all documents of this model
		prefix := b.generateKey("project_doc", project.ID, modelName)
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		var keysToDelete [][]byte
		prefixBytes := []byte(prefix)
		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			key := it.Item().Key()
			keysToDelete = append(keysToDelete, append([]byte(nil), key...))
		}

		// Delete all collected keys
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}

		return nil
	})
}

// CreateIndex creates an index for a model in the project (no-op for BadgerDB)
func (b *BadgerDriver) CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parent_field string) error {
	// BadgerDB doesn't support secondary indexes directly
	return nil
}

// DropIndex drops an index from a model in the project (no-op for BadgerDB)
func (b *BadgerDriver) DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error {
	// BadgerDB doesn't support secondary indexes directly
	return nil
}

// GetSingleProjectDocument retrieves a single document by its ID
func (b *BadgerDriver) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	docKey := b.generateKey("project_doc", param.ProjectID, param.Model.Name, param.DocumentID)

	docJSON, err := b.getValue(docKey)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, fmt.Errorf("document not found")
		}
		return nil, err
	}

	var doc types.DefaultDocumentStructure
	err = json.Unmarshal(docJSON, &doc)
	if err != nil {
		return nil, err
	}

	return &doc, nil
}

// GetSingleProjectDocumentBytes retrieves a single document as bytes
func (b *BadgerDriver) GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	doc, err := b.GetSingleProjectDocument(ctx, param)
	if err != nil {
		return nil, err
	}

	return json.Marshal(doc)
}

// GetSingleProjectDocumentRevisions retrieves document revisions
func (b *BadgerDriver) GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
	prefix := b.generateKey("project_rev", param.ProjectID, param.DocumentID)

	revisions, err := b.getAllWithPrefix(prefix)
	if err != nil {
		return nil, err
	}

	var result []*types.DefaultDocumentStructure
	for _, revisionData := range revisions {
		var revision types.DefaultDocumentStructure
		if err := json.Unmarshal(revisionData, &revision); err != nil {
			continue
		}
		result = append(result, &revision)
	}

	return result, nil
}

// GetSingleRawDocumentFromProject retrieves a raw document
func (b *BadgerDriver) GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	return b.GetSingleProjectDocument(ctx, param)
}

// QueryMultiDocumentOfProject retrieves multiple documents from a project
func (b *BadgerDriver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
	prefix := b.generateKey("project_doc", param.ProjectID, param.Model.Name)

	documents, err := b.getAllWithPrefix(prefix)
	if err != nil {
		return nil, err
	}

	var result []*types.DefaultDocumentStructure
	for _, docData := range documents {
		var doc types.DefaultDocumentStructure
		if err := json.Unmarshal(docData, &doc); err != nil {
			continue
		}
		result = append(result, &doc)
	}

	return result, nil
}

// QueryMultiDocumentOfProjectBytes retrieves multiple documents as bytes
func (b *BadgerDriver) QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	docs, err := b.QueryMultiDocumentOfProject(ctx, param)
	if err != nil {
		return nil, err
	}

	return json.Marshal(docs)
}

// AddDocumentToProject adds a new document to the project
func (b *BadgerDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
	// Initialize metadata if not exists
	if err := b.initializeMetadata(ctx, param.ProjectID); err != nil {
		return nil, err
	}

	// Generate ID if not provided
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}

	// Initialize Meta field if nil
	if doc.Meta == nil {
		doc.Meta = &types.MetaField{}
	}

	doc.Meta.CreatedAt = time.Now().Format(time.RFC3339)
	doc.Meta.UpdatedAt = time.Now().Format(time.RFC3339)

	documentJSON, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	docKey := b.generateKey("project_doc", param.ProjectID, param.Model.Name, doc.ID)
	if err := b.setValue(docKey, documentJSON); err != nil {
		return nil, err
	}

	return doc, nil
}

// UpdateDocumentOfProject updates a particular document in the project
func (b *BadgerDriver) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {
	// Initialize Meta field if nil
	if doc.Meta == nil {
		doc.Meta = &types.MetaField{}
	}

	doc.Meta.UpdatedAt = time.Now().Format(time.RFC3339)

	documentJSON, err := json.Marshal(doc)
	if err != nil {
		return err
	}

	docKey := b.generateKey("project_doc", param.ProjectID, param.Model.Name, doc.ID)
	return b.setValue(docKey, documentJSON)
}

// DeleteDocumentFromProject deletes a document from the project
func (b *BadgerDriver) DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	docKey := b.generateKey("project_doc", param.ProjectID, param.Model.Name, param.DocumentID)
	return b.deleteKey(docKey)
}

// DeleteDocumentsFromProject deletes multiple documents from the project
func (b *BadgerDriver) DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	return b.DB.Update(func(txn *badger.Txn) error {
		for _, docID := range param.DocumentIDs {
			docKey := b.generateKey("project_doc", param.ProjectID, param.Model.Name, docID)
			if err := txn.Delete([]byte(docKey)); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
		return nil
	})
}

// DeleteDocumentRelation deletes all relations or data in pivot tables from the project
func (b *BadgerDriver) DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error {
	return b.DB.Update(func(txn *badger.Txn) error {
		// Delete relations where this document is the source or target
		prefix := b.generateKey("project_rel", param.ProjectID)
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		var keysToDelete [][]byte
		prefixBytes := []byte(prefix)
		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			key := it.Item().Key()

			err := it.Item().Value(func(val []byte) error {
				var relation map[string]interface{}
				if err := json.Unmarshal(val, &relation); err != nil {
					return nil // Skip invalid relations
				}

				// Check if this relation involves the document
				if fromId, ok := relation["from_id"]; ok && fromId == param.DocumentID {
					keysToDelete = append(keysToDelete, append([]byte(nil), key...))
				} else if toId, ok := relation["to_id"]; ok && toId == param.DocumentID {
					keysToDelete = append(keysToDelete, append([]byte(nil), key...))
				}

				return nil
			})

			if err != nil {
				return err
			}
		}

		// Delete all collected keys
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}

		return nil
	})
}

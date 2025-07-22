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

// AddRelationFields adds relation fields to a model (no-op for BadgerDB)
func (b *BadgerDriver) AddRelationFields(ctx context.Context, param *models.CommonSystemParams, sourceModel, targetModel, relationType string) error {
	// BadgerDB handles relations through separate key-value pairs, no schema changes needed
	return nil
}

// DeleteRelationDocuments deletes relation documents for a specific relation
func (b *BadgerDriver) DeleteRelationDocuments(ctx context.Context, param *models.CommonSystemParams, relationshipName string) error {
	return b.DB.Update(func(txn *badger.Txn) error {
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

				// Check if this relation matches the relationship name
				if relName, ok := relation["relation_name"]; ok && relName == relationshipName {
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

// GetRelationDocument retrieves a relation document
func (b *BadgerDriver) GetRelationDocument(ctx context.Context, cd *models.ConnectDisconnectParam) (*types.DefaultDocumentStructure, error) {
	relKey := b.generateKey("project_rel", cd.DocCollectionName, cd.CurrentActionID)

	relJSON, err := b.getValue(relKey)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, fmt.Errorf("relation document not found")
		}
		return nil, err
	}

	var doc types.DefaultDocumentStructure
	err = json.Unmarshal(relJSON, &doc)
	if err != nil {
		return nil, err
	}

	return &doc, nil
}

// CreateRelation creates a new relation between documents
func (b *BadgerDriver) CreateRelation(ctx context.Context, param *models.CommonSystemParams, relation *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	// Initialize metadata if not exists
	if err := b.initializeMetadata(ctx, param.ProjectID); err != nil {
		return nil, err
	}

	// Generate unique relation ID if not provided
	if relation.ID == "" {
		relation.ID = uuid.New().String()
	}

	relationJSON, err := json.Marshal(relation)
	if err != nil {
		return nil, err
	}

	relKey := b.generateKey("project_rel", param.ProjectID, relation.ID)
	if err := b.setValue(relKey, relationJSON); err != nil {
		return nil, err
	}

	return relation, nil
}

// DeleteRelation deletes a specific relation
func (b *BadgerDriver) DeleteRelation(ctx context.Context, param *models.CommonSystemParams, relationID string) error {
	relKey := b.generateKey("project_rel", param.ProjectID, relationID)
	return b.deleteKey(relKey)
}

// NewInsertableRelations creates new insertable relations
func (b *BadgerDriver) NewInsertableRelations(ctx context.Context, param *models.CommonSystemParams, relations []*types.DefaultDocumentStructure) ([]*types.DefaultDocumentStructure, error) {
	var createdRelations []*types.DefaultDocumentStructure

	for _, relation := range relations {
		createdRelation, err := b.CreateRelation(ctx, param, relation)
		if err != nil {
			return nil, err
		}
		createdRelations = append(createdRelations, createdRelation)
	}

	return createdRelations, nil
}

// CheckOneToOneRelationExists checks if a one-to-one relation already exists
func (b *BadgerDriver) CheckOneToOneRelationExists(ctx context.Context, param *models.CommonSystemParams, fromId, toId, relationName string) (bool, error) {
	prefix := b.generateKey("project_rel", param.ProjectID)
	relations, err := b.getAllWithPrefix(prefix)
	if err != nil {
		return false, err
	}

	for _, relationData := range relations {
		var relation map[string]interface{}
		if err := json.Unmarshal(relationData, &relation); err != nil {
			continue
		}

		// Check if this relation matches our criteria
		if relName, ok := relation["relation_name"]; ok && relName == relationName {
			if fId, ok := relation["from_id"]; ok && fId == fromId {
				if tId, ok := relation["to_id"]; ok && tId == toId {
					return true, nil
				}
			}
		}
	}

	return false, nil
}

// GetRelationIds gets relation IDs for a document
func (b *BadgerDriver) GetRelationIds(ctx context.Context, param *models.CommonSystemParams, documentID, relationName string) ([]string, error) {
	prefix := b.generateKey("project_rel", param.ProjectID)
	relations, err := b.getAllWithPrefix(prefix)
	if err != nil {
		return nil, err
	}

	var relationIds []string
	for _, relationData := range relations {
		var relation map[string]interface{}
		if err := json.Unmarshal(relationData, &relation); err != nil {
			continue
		}

		// Check if this relation matches our criteria
		if relName, ok := relation["relation_name"]; ok && relName == relationName {
			if fId, ok := relation["from_id"]; ok && fId == documentID {
				if tId, ok := relation["to_id"]; ok {
					if toIdStr, ok := tId.(string); ok {
						relationIds = append(relationIds, toIdStr)
					}
				}
			}
		}
	}

	return relationIds, nil
}

// ConnectBuilder connects a builder to the project
func (b *BadgerDriver) ConnectBuilder(ctx context.Context, projectId, userId string) error {
	builderKey := b.generateKey("project_builder", projectId, userId)
	builderData := map[string]interface{}{
		"project_id":   projectId,
		"user_id":      userId,
		"connected":    true,
		"connected_at": time.Now().Format(time.RFC3339),
	}

	builderJSON, err := json.Marshal(builderData)
	if err != nil {
		return err
	}

	return b.setValue(builderKey, builderJSON)
}

// DisconnectBuilder disconnects a builder from the project
func (b *BadgerDriver) DisconnectBuilder(ctx context.Context, projectId, userId string) error {
	builderKey := b.generateKey("project_builder", projectId, userId)
	return b.deleteKey(builderKey)
}

// GetProjectUser gets a project user by email or phone
func (b *BadgerDriver) GetProjectUser(ctx context.Context, projectId, email, phone string) (*types.DefaultDocumentStructure, error) {
	prefix := b.generateKey("project_user", projectId)
	users, err := b.getAllWithPrefix(prefix)
	if err != nil {
		return nil, err
	}

	for _, userData := range users {
		var user types.DefaultDocumentStructure
		if err := json.Unmarshal(userData, &user); err != nil {
			continue
		}

		// Check if user matches email or phone
		if user.Data != nil {
			if email != "" {
				if userEmail, ok := user.Data["email"]; ok && userEmail == email {
					return &user, nil
				}
			}
			if phone != "" {
				if userPhone, ok := user.Data["phone"]; ok && userPhone == phone {
					return &user, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("user not found")
}

// GetLoggedInProjectUser gets a logged-in project user by user ID
func (b *BadgerDriver) GetLoggedInProjectUser(ctx context.Context, projectId, userId string) (*types.DefaultDocumentStructure, error) {
	userKey := b.generateKey("project_user", projectId, userId)

	userJSON, err := b.getValue(userKey)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	var user types.DefaultDocumentStructure
	err = json.Unmarshal(userJSON, &user)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// GetProjectUsers gets project users by their IDs
func (b *BadgerDriver) GetProjectUsers(ctx context.Context, projectId string, userIds []string) ([]*types.DefaultDocumentStructure, error) {
	var users []*types.DefaultDocumentStructure

	for _, userId := range userIds {
		userKey := b.generateKey("project_user", projectId, userId)
		userJSON, err := b.getValue(userKey)
		if err != nil {
			continue // Skip errors, continue with other users
		}

		var user types.DefaultDocumentStructure
		err = json.Unmarshal(userJSON, &user)
		if err == nil {
			users = append(users, &user)
		}
	}

	return users, nil
}

// GetAllRelationDocumentsOfSingleDocument gets all relation documents for a single document
func (b *BadgerDriver) GetAllRelationDocumentsOfSingleDocument(ctx context.Context, param *models.CommonSystemParams, documentID string) ([]*types.DefaultDocumentStructure, error) {
	prefix := b.generateKey("project_rel", param.ProjectID)
	relations, err := b.getAllWithPrefix(prefix)
	if err != nil {
		return nil, err
	}

	var result []*types.DefaultDocumentStructure
	for _, relationData := range relations {
		var relation types.DefaultDocumentStructure
		if err := json.Unmarshal(relationData, &relation); err != nil {
			continue
		}

		// Check if this relation involves the document
		if relation.Data != nil {
			if fromId, ok := relation.Data["from_id"]; ok && fromId == documentID {
				result = append(result, &relation)
			} else if toId, ok := relation.Data["to_id"]; ok && toId == documentID {
				result = append(result, &relation)
			}
		}
	}

	return result, nil
}

// AddTeamMetaInfo adds team meta info to a document
func (b *BadgerDriver) AddTeamMetaInfo(ctx context.Context, param *models.CommonSystemParams, teamInfo *types.DefaultDocumentStructure) (*types.DefaultDocumentStructure, error) {
	// Store team info as a regular document
	result, err := b.AddDocumentToProject(ctx, param, teamInfo)
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
func (b *BadgerDriver) RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, ids []string) ([]*types.DefaultDocumentStructure, error) {
	var relations []*types.DefaultDocumentStructure

	for _, id := range ids {
		relKey := b.generateKey("project_rel", param.ProjectID, id)
		relJSON, err := b.getValue(relKey)
		if err != nil {
			continue // Skip errors
		}

		var relation types.DefaultDocumentStructure
		err = json.Unmarshal(relJSON, &relation)
		if err == nil {
			relations = append(relations, &relation)
		}
	}

	return relations, nil
}

// RelationshipDataLoaderBytes loads relationship data as bytes
func (b *BadgerDriver) RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, ids []string) ([][]byte, error) {
	var relationBytes [][]byte

	for _, id := range ids {
		relKey := b.generateKey("project_rel", param.ProjectID, id)
		relJSON, err := b.getValue(relKey)
		if err != nil {
			continue // Skip errors
		}

		relationBytes = append(relationBytes, relJSON)
	}

	return relationBytes, nil
}

// CountDocOfProject counts documents in a project
func (b *BadgerDriver) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (int, error) {
	prefix := b.generateKey("project_doc", param.ProjectID, param.Model.Name)
	documents, err := b.getAllWithPrefix(prefix)
	if err != nil {
		return 0, err
	}

	return len(documents), nil
}

// CountDocOfProjectBytes counts documents in a project (bytes version)
func (b *BadgerDriver) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) (int, error) {
	return b.CountDocOfProject(ctx, param)
}

// CountMultiDocumentOfProject counts multiple documents with filters
func (b *BadgerDriver) CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, condition map[string]interface{}) (int, error) {
	// For simplicity, return total count (BadgerDB filtering would require more complex implementation)
	return b.CountDocOfProject(ctx, param)
}

// AggregateDocOfProject performs aggregation on documents
func (b *BadgerDriver) AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams, pipeline interface{}) (interface{}, error) {
	// BadgerDB doesn't support complex aggregation pipelines like MongoDB
	// This would need custom implementation based on specific aggregation needs
	return nil, fmt.Errorf("aggregation not yet implemented for BadgerDB")
}

// AggregateDocOfProjectBytes performs aggregation on documents (bytes version)
func (b *BadgerDriver) AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams, pipeline interface{}) ([]byte, error) {
	result, err := b.AggregateDocOfProject(ctx, param, pipeline)
	if err != nil {
		return nil, err
	}

	return json.Marshal(result)
}

// DropField drops a field from all documents in a collection
func (b *BadgerDriver) DropField(ctx context.Context, param *models.CommonSystemParams, fieldName string) error {
	prefix := b.generateKey("project_doc", param.ProjectID, param.Model.Name)
	documents, err := b.getAllWithPrefix(prefix)
	if err != nil {
		return err
	}

	return b.DB.Update(func(txn *badger.Txn) error {
		for key, docData := range documents {
			var doc types.DefaultDocumentStructure
			err = json.Unmarshal(docData, &doc)
			if err != nil {
				continue
			}

			// Remove the field
			if doc.Data != nil {
				delete(doc.Data, fieldName)
			}

			// Update the updated_at timestamp
			if doc.Meta == nil {
				doc.Meta = &types.MetaField{}
			}
			doc.Meta.UpdatedAt = time.Now().Format(time.RFC3339)

			// Save back
			updatedDocJSON, err := json.Marshal(doc)
			if err != nil {
				continue
			}

			if err := txn.Set([]byte(key), updatedDocJSON); err != nil {
				return err
			}
		}
		return nil
	})
}

// RenameField renames a field in all documents in a collection
func (b *BadgerDriver) RenameField(ctx context.Context, param *models.CommonSystemParams, oldFieldName, newFieldName string) error {
	prefix := b.generateKey("project_doc", param.ProjectID, param.Model.Name)
	documents, err := b.getAllWithPrefix(prefix)
	if err != nil {
		return err
	}

	return b.DB.Update(func(txn *badger.Txn) error {
		for key, docData := range documents {
			var doc types.DefaultDocumentStructure
			err = json.Unmarshal(docData, &doc)
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

			// Update the updated_at timestamp
			if doc.Meta == nil {
				doc.Meta = &types.MetaField{}
			}
			doc.Meta.UpdatedAt = time.Now().Format(time.RFC3339)

			// Save back
			updatedDocJSON, err := json.Marshal(doc)
			if err != nil {
				continue
			}

			if err := txn.Set([]byte(key), updatedDocJSON); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteMediaFile deletes a media file document
func (b *BadgerDriver) DeleteMediaFile(ctx context.Context, param *models.CommonSystemParams) error {
	return b.DeleteDocumentFromProject(ctx, param)
}

// DuplicateModel duplicates a model by copying all its documents
func (b *BadgerDriver) DuplicateModel(ctx context.Context, param *models.CommonSystemParams, newModelName string) error {
	prefix := b.generateKey("project_doc", param.ProjectID, param.Model.Name)
	documents, err := b.getAllWithPrefix(prefix)
	if err != nil {
		return err
	}

	return b.DB.Update(func(txn *badger.Txn) error {
		for _, docData := range documents {
			var doc types.DefaultDocumentStructure
			err = json.Unmarshal(docData, &doc)
			if err != nil {
				continue
			}

			// Create new document with new ID
			doc.ID = uuid.New().String()

			// Update timestamps
			if doc.Meta == nil {
				doc.Meta = &types.MetaField{}
			}
			doc.Meta.CreatedAt = time.Now().Format(time.RFC3339)
			doc.Meta.UpdatedAt = time.Now().Format(time.RFC3339)

			newDocJSON, err := json.Marshal(doc)
			if err != nil {
				continue
			}

			newDocKey := b.generateKey("project_doc", param.ProjectID, newModelName, doc.ID)
			if err := txn.Set([]byte(newDocKey), newDocJSON); err != nil {
				return err
			}
		}
		return nil
	})
}

package bbolt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	apitobolt "github.com/apito-io/apitoBolt"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
)

// ============================================================================
// HELPER METHODS - Collection Naming Strategy (Mongo-style)
// ============================================================================

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaults string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaults
}

// getCollectionName returns the collection name for a model
// Pattern: "p_{projectId}"
func (b *BBoltDriver) getCollectionName(projectID, modelName string) string {
	return fmt.Sprintf("p_%s", projectID)
}

// getRelationCollectionName returns the collection name for relations
// Pattern: "relation_{fromModel}_{toModel}"
func (b *BBoltDriver) getRelationCollectionName(fromModel, toModel string) string {
	return fmt.Sprintf("relation_%s_%s", fromModel, toModel)
}

// getMediaCollectionName returns the collection name for media files
// Pattern: "p_{projectId}_files"
func (b *BBoltDriver) getMediaCollectionName(projectID string) string {
	return fmt.Sprintf("p_%s_files", projectID)
}

// searchAndAppend is a helper function to recursively search and append fields to nested structures
func (b *BBoltDriver) searchAndAppend(fields []*models.FieldInfo, isUpdate bool, parentName string, singleField *models.FieldInfo, depth int) int {
	for _, field := range fields {
		if field.Identifier == parentName {
			if isUpdate {
				// Check if a matching field exists in SubFields by ID
				for i, subField := range field.SubFieldInfo {
					if subField.Identifier == singleField.Identifier {
						// Replace the existing field
						field.SubFieldInfo[i] = singleField
						return depth
					}
				}
			}
			// Append the singleField if not updating
			field.SubFieldInfo = append(field.SubFieldInfo, singleField)
			return depth
		}
		// Recurse into SubFields
		if subDepth := b.searchAndAppend(field.SubFieldInfo, isUpdate, parentName, singleField, depth+1); subDepth > 0 {
			return subDepth
		}
	}
	return 0 // Parent not found
}

// ============================================================================
// PROJECT MANAGEMENT
// ============================================================================

// DeleteProject implements the deletion of a project
func (b *BBoltDriver) DeleteProject(ctx context.Context, projectId string) error {
	// Define collection names
	projectCollection := fmt.Sprintf("p_%s", projectId)
	projectMediaCollection := fmt.Sprintf("%s_files", projectCollection)
	projectEdgeCollection := fmt.Sprintf("%s_relation", projectCollection)

	// Execute operations in a transaction
	return b.Store.Update(func(tx *apitobolt.Tx) error {
		// Drop the main project collection
		if err := tx.Collection(projectCollection).Drop(); err != nil {
			// Ignore errors if collection doesn't exist
		}

		// Drop the media collection
		if err := tx.Collection(projectMediaCollection).Drop(); err != nil {
			// Ignore errors if collection doesn't exist
		}

		// Drop the relation collection
		if err := tx.Collection(projectEdgeCollection).Drop(); err != nil {
			// Ignore errors if collection doesn't exist
		}

		return nil
	})
}

// TransferProject transfers a project from one user to another
func (b *BBoltDriver) TransferProject(ctx context.Context, userId, from, to string) error {
	// Transfer main documents
	err := b.transferDocs(ctx, userId, from, to)
	if err != nil {
		return err
	}

	// Transfer relation documents
	err = b.transferRelationDocs(ctx, from, to)
	if err != nil {
		return err
	}

	// Transfer media documents
	err = b.transferMediaDocs(ctx, from, to)
	if err != nil {
		return err
	}

	return nil
}

// transferDocs transfers documents from one project to another
func (b *BBoltDriver) transferDocs(ctx context.Context, userId, from, to string) error {
	// Source collection name
	fromCollectionName := "p_" + from

	// Get all documents from source collection
	var docs []types.DefaultDocumentStructure
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(fromCollectionName)
		return col.All(&docs)
	})
	if err != nil {
		return err
	}

	// Target collection name
	toCollectionName := "p_" + to

	// Insert documents into target collection
	return b.Store.Update(func(tx *apitobolt.Tx) error {
		toCol := tx.Collection(toCollectionName)

		for _, doc := range docs {
			// Update metadata
			doc.Meta.CreatedAt = utility.GetCurrentTime()
			doc.Meta.UpdatedAt = utility.GetCurrentTime()
			doc.Meta.CreatedBy = &types.SystemUser{ID: userId}
			doc.Meta.LastModifiedBy = &types.SystemUser{ID: userId}

			if _, err := toCol.Save(&doc); err != nil {
				return err
			}
		}

		return nil
	})
}

// transferMediaDocs transfers media documents from one project to another
func (b *BBoltDriver) transferMediaDocs(ctx context.Context, from, to string) error {
	// Source collection name
	fromCollectionName := "p_" + from + "_files"

	// Get all documents from source collection
	var docs []models.FileDetails
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(fromCollectionName)
		return col.All(&docs)
	})
	if err != nil {
		return err
	}

	// Target collection name
	toCollectionName := "p_" + to + "_files"

	// Insert documents into target collection
	return b.Store.Update(func(tx *apitobolt.Tx) error {
		toCol := tx.Collection(toCollectionName)

		for _, doc := range docs {
			// Update the URL path
			doc.URL = strings.Replace(doc.URL, from, to, -1)
			if doc.UploadParam != nil {
				doc.UploadParam.ProjectID = to
			}

			// Update metadata
			doc.CreatedAt = utility.GetCurrentTime()

			if _, err := toCol.Save(&doc); err != nil {
				return err
			}
		}

		return nil
	})
}

// transferRelationDocs transfers relation documents from one project to another
func (b *BBoltDriver) transferRelationDocs(ctx context.Context, from, to string) error {
	// Source collection name
	fromCollectionName := "p_" + from + "_relation"

	// Get all documents from source collection
	var docs []models.EdgeRelation
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(fromCollectionName)
		return col.All(&docs)
	})
	if err != nil {
		return err
	}

	// Target collection name
	toCollectionName := "p_" + to + "_relation"

	// Insert documents into target collection
	return b.Store.Update(func(tx *apitobolt.Tx) error {
		toCol := tx.Collection(toCollectionName)

		for _, doc := range docs {
			// Update the from and to relations
			doc.XFrom = strings.Replace(doc.XFrom, from, to, -1)
			doc.XTo = strings.Replace(doc.XTo, from, to, -1)

			// Update metadata
			doc.CreatedAt = utility.GetCurrentTime()

			if _, err := toCol.Save(&doc); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetProject retrieves a project by ID (not part of ProjectDBInterface, but useful)
func (b *BBoltDriver) GetProject(ctx context.Context, id string) (*models.Project, error) {
	// This would typically query system DB, not project DB
	return nil, errors.New("GetProject not implemented - use system DB")
}

// ============================================================================
// COLLECTION MANAGEMENT
// ============================================================================

func (b *BBoltDriver) InitProjectBase(ctx context.Context, param *models.CommonSystemParams, indexes []string) error {
	return nil
}

// DeleteProjectBase is a no-op for BBolt (InitProjectBase does not allocate base storage here).
func (b *BBoltDriver) DeleteProjectBase(ctx context.Context, param *models.CommonSystemParams) error {
	return nil
}

// CheckTableOrCollectionExists checks if a table or collection exists in the project
func (b *BBoltDriver) CheckTableOrCollectionExists(ctx context.Context, param *models.CommonSystemParams) (bool, error) {
	collectionName := param.Model.Name

	// Try to access collection and check if it has any documents
	var count int
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		var docs []map[string]interface{}
		if err := col.All(&docs); err != nil {
			return err
		}
		count = len(docs)
		return nil
	})

	if err != nil {
		return false, nil
	}

	return count > 0, nil
}

// CreateTableOrCollection creates a new table or collection in the project
func (b *BBoltDriver) CreateTableOrCollection(ctx context.Context, param *models.CommonSystemParams, indexes []string) error {
	var collectionName string
	if param.Model != nil {
		collectionName = param.Model.Name
	} else {
		return nil
	}

	if collectionName == "" {
		return errors.New("collection name must be provided")
	}

	// Create the collection and indexes
	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)

		// Initialize collection
		if err := col.Init(); err != nil {
			return err
		}

		// Create indexes for relation collections
		indexes := []string{"type", "id"}
		for _, idx := range indexes {
			if err := col.EnsureIndex(idx, false); err != nil {
				return err
			}
		}

		return nil
	})
}

// ============================================================================
// MODEL MANAGEMENT
// ============================================================================

// AddModel adds a new model to the project
func (b *BBoltDriver) AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error) {
	if model.SinglePage {
		model.SinglePageUUID = utility.NewID()
	}

	if project.Schema == nil {
		project.Schema = &models.ProjectSchema{
			Models: []*models.ModelType{model},
		}
	} else {
		for _, ct := range project.Schema.Models {
			if ct.Name == model.Name {
				return nil, errors.New("model Already Defined")
			}
		}
		project.Schema.Models = append(project.Schema.Models, model)
	}

	collectionName := model.Name
	err := b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		if err := col.Init(); err != nil {
			return err
		}
		// Create basic index on id
		return col.EnsureIndex("id", false)
	})

	if err != nil {
		return nil, err
	}

	return project.Schema, nil
}

// AddFieldToModel adds a new field to an existing model in the project
func (b *BBoltDriver) AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parent_field string) (*models.ModelType, error) {
	// Since we don't have direct access to project schema, we'll work with the provided model
	if param.Model == nil {
		return nil, errors.New("model not provided")
	}

	targetModel := param.Model

	// Handle field addition based on whether it's a parent field or nested field
	if parent_field == "" {
		// Adding a top-level field
		if isUpdate {
			// Update existing field
			for i, field := range targetModel.Fields {
				if field.Identifier == param.FieldInfo.Identifier {
					targetModel.Fields[i] = param.FieldInfo
					return targetModel, nil
				}
			}
		}
		for _, existing := range targetModel.Fields {
			if existing != nil && existing.Identifier == param.FieldInfo.Identifier {
				return targetModel, nil
			}
		}
		// Add new field
		targetModel.Fields = append(targetModel.Fields, param.FieldInfo)
	} else {
		// Adding to a nested/repeated group
		depth := b.searchAndAppend(targetModel.Fields, isUpdate, parent_field, param.FieldInfo, 0)
		if depth == 0 {
			return nil, errors.New("parent field not found")
		}
	}

	return targetModel, nil
}

// RenameModel renames a model in the project
func (b *BBoltDriver) RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error {
	// Check if the new name already exists
	for _, model := range project.Schema.Models {
		if model.Name == newName {
			return errors.New("model with this name already exists")
		}
	}

	// Find the model to rename
	var modelToRename *models.ModelType
	for _, model := range project.Schema.Models {
		if model.Name == modelName {
			modelToRename = model
			break
		}
	}

	if modelToRename == nil {
		return errors.New("model not found")
	}

	// Rename the collection
	oldCollection := modelName
	newCollection := newName

	err := b.Store.Update(func(tx *apitobolt.Tx) error {
		oldCol := tx.Collection(oldCollection)
		newCol := tx.Collection(newCollection)

		// Initialize new collection
		if err := newCol.Init(); err != nil {
			return err
		}

		// Get all documents from old collection
		var docs []types.DefaultDocumentStructure
		if err := oldCol.All(&docs); err != nil {
			return err
		}

		// Transfer documents to new collection
		for _, doc := range docs {
			if _, err := newCol.Save(&doc); err != nil {
				return err
			}
		}

		// Drop old collection
		return oldCol.Drop()
	})

	if err != nil {
		return err
	}

	// Update model name in schema
	modelToRename.Name = newName

	return nil
}

// ConvertModel converts a model in the project (no-op for BBolt)
func (b *BBoltDriver) ConvertModel(ctx context.Context, project *models.Project, modelName string) error {
	return nil
}

// DropModel drops a model from the project
func (b *BBoltDriver) DropModel(ctx context.Context, project *models.Project, modelName string) error {
	// Find and remove the model from schema
	var updatedModels []*models.ModelType
	for _, model := range project.Schema.Models {
		if model.Name != modelName {
			updatedModels = append(updatedModels, model)
		}
	}

	if len(updatedModels) == len(project.Schema.Models) {
		return errors.New("model not found")
	}

	project.Schema.Models = updatedModels

	// Drop the collection
	collectionName := modelName
	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		return col.Drop()
	})
}

// ============================================================================
// INDEX MANAGEMENT
// ============================================================================

// CreateIndex creates an index on a field
func (b *BBoltDriver) CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parent_field string) error {
	collectionName := param.Model.Name

	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		return col.EnsureIndex(fieldName, false)
	})
}

// DropIndex drops an index from a model (no-op for ApitoBolt)
func (b *BBoltDriver) DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error {
	// ApitoBolt doesn't require explicit index dropping
	return nil
}

// ============================================================================
// RELATION OPERATIONS
// ============================================================================

// AddRelationFields creates a relation field (has one or has many) between models (no-op for BBolt)
func (b *BBoltDriver) AddRelationFields(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error {
	return nil
}

// DeleteRelationDocuments drops pivot tables, relation keys, or collection tables
func (b *BBoltDriver) DeleteRelationDocuments(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error {
	collectionName := fmt.Sprintf("relation_%s_%s", from.Model, to.Model)

	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		return col.Drop()
	})
}

// GetRelationDocument retrieves a relation document by ID
func (b *BBoltDriver) GetRelationDocument(ctx context.Context, param *models.ConnectDisconnectParam) (*models.EdgeRelation, error) {
	collectionName := param.DocCollectionName

	var relation models.EdgeRelation
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		return col.FindByID(param.CurrentActionID, &relation)
	})

	if err != nil {
		return nil, fmt.Errorf("relation document not found: %v", err)
	}

	return &relation, nil
}

// CreateRelation creates a relation in the project
func (b *BBoltDriver) CreateRelation(ctx context.Context, projectId string, relation *models.EdgeRelation) error {
	collectionName := fmt.Sprintf("p_%s_relation", projectId)

	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		// Initialize if needed
		if err := col.Init(); err != nil {
			return err
		}
		_, err := col.Save(relation)
		return err
	})
}

// DeleteRelation deletes a relation in the project
func (b *BBoltDriver) DeleteRelation(ctx context.Context, param *models.ConnectDisconnectParam, id string) error {
	collectionName := param.DocRelationName

	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		return col.DeleteStruct(&models.EdgeRelation{Key: id})
	})
}

// NewInsertableRelations retrieves new insertable relations in the project
func (b *BBoltDriver) NewInsertableRelations(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	relationCollectionName := fmt.Sprintf("relation_%s_%s", param.ForwardConnectionType.Model, param.BackwardConnectionType.Model)

	// Find existing relations for this document
	var existingRelations = make(map[string]bool)
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(relationCollectionName)
		var relations []models.EdgeRelation
		if err := col.All(&relations); err != nil {
			return err
		}

		for _, relation := range relations {
			if relation.FromID == param.ForwardConnectionID {
				existingRelations[relation.ToID] = true
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Filter out existing relations from action IDs
	var newRelations []string
	for _, actionID := range param.ActionIDs {
		if !existingRelations[actionID] {
			newRelations = append(newRelations, actionID)
		}
	}

	return newRelations, nil
}

// CheckOneToOneRelationExists checks if a one-to-one relation exists
func (b *BBoltDriver) CheckOneToOneRelationExists(ctx context.Context, param *models.ConnectDisconnectParam) (bool, error) {
	relationCollectionName := fmt.Sprintf("relation_%s_%s", param.ForwardConnectionType.Model, param.BackwardConnectionType.Model)

	var count int
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(relationCollectionName)
		var relations []models.EdgeRelation
		if err := col.All(&relations); err != nil {
			return err
		}

		for _, relation := range relations {
			if relation.FromID == param.ForwardConnectionID {
				count++
			}
		}

		return nil
	})

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetRelationIds retrieves the IDs of every document related to a document
func (b *BBoltDriver) GetRelationIds(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	relationCollectionName := fmt.Sprintf("relation_%s_%s", param.ForwardConnectionType.Model, param.BackwardConnectionType.Model)

	var relationIds []string
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(relationCollectionName)
		var relations []models.EdgeRelation
		if err := col.All(&relations); err != nil {
			return err
		}

		for _, relation := range relations {
			if relation.FromID == param.ForwardConnectionID {
				relationIds = append(relationIds, relation.ToID)
			}
		}

		return nil
	})

	return relationIds, err
}

// ============================================================================
// BUILDER OPERATIONS
// ============================================================================

// ConnectBuilder connects a builder to the project
func (b *BBoltDriver) ConnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	// Process each connection/disconnect parameter
	if param.ConDisParam == nil || len(param.ConDisParam) == 0 {
		return nil
	}

	for _, connParam := range param.ConDisParam {
		var filteredIDs []string
		for _, id := range connParam.ActionIDs {
			if strings.TrimSpace(id) != "" {
				filteredIDs = append(filteredIDs, id)
			}
		}

		if len(filteredIDs) == 0 {
			continue
		}

		// before creating relation check if its already exists or not
		newRelations, err := b.NewInsertableRelations(ctx, connParam)
		if err != nil {
			return err
		}
		if len(newRelations) == 0 {
			continue
		}

		// add a new relation for each action ID
		for _, id := range connParam.ActionIDs {
			fromID := connParam.ForwardConnectionID
			toID := id

			if connParam.ConnectionType == "backward" {
				fromID = id
				toID = connParam.ForwardConnectionID
			}

			relation := &models.EdgeRelation{
				Key:       utility.NewID(),
				To:        connParam.ForwardConnectionType.Model,
				From:      connParam.BackwardConnectionType.Model,
				Relation:  connParam.ForwardConnectionType.Relation,
				FromID:    fromID,
				ToID:      toID,
				CreatedAt: utility.GetCurrentTime(),
			}
			if connParam.KnownAs != "" {
				relation.KnownAs = connParam.KnownAs
			}

			err := b.CreateRelation(ctx, param.ProjectID, relation)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DisconnectBuilder disconnects a builder from the project
func (b *BBoltDriver) DisconnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	if param.ConDisParam == nil || len(param.ConDisParam) == 0 {
		return nil
	}

	for _, connParam := range param.ConDisParam {
		if len(connParam.ActionIDs) > 0 {
			var filteredIDs []string
			for _, id := range connParam.ActionIDs {
				if id != "" {
					filteredIDs = append(filteredIDs, id)
				}
			}

			if len(filteredIDs) == 0 {
				continue
			}

			for _, _id := range filteredIDs {
				connParam.CurrentActionID = _id
				err := b.DeleteRelation(ctx, connParam, connParam.CurrentActionID)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// ============================================================================
// DOCUMENT OPERATIONS
// ============================================================================

// GetSingleProjectDocument gets a single document from a project
func (b *BBoltDriver) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	// Handle intersection results
	if param.ResolveParams != nil {
		if val, ok := param.ResolveParams.Args["intersect"].(bool); ok {
			param.IsIntersectionResult = val
		}
	}

	// Get collection name
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	// Query the document
	var doc types.DefaultDocumentStructure
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		if err := col.FindByID(param.DocumentID, &doc); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, errors.New("no result found with this request id")
	}

	if !bboltDocPassesQueryHook(b.Conf, param, &doc) {
		return nil, errors.New("no result found with this request id")
	}

	// Handle payload for non-system requests
	if !param.IsSystemRequest {
		utility.HandlePayload(param.Model, doc.Data)
	}

	return &doc, nil
}

// GetSingleProjectDocumentBytes retrieves a single project document by ID as bytes
func (b *BBoltDriver) GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	doc, err := b.GetSingleProjectDocument(ctx, param)
	if err != nil {
		return nil, err
	}

	bytes, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}

	if debug := getEnv("DEBUG", "false"); debug == "true" {
		fmt.Println(fmt.Sprintf("Project : %s | Model : %s | Document : %s | Result : %s", param.ProjectID, param.Model.Name, param.DocumentID, string(bytes)))
	}

	return bytes, nil
}

// GetSingleProjectDocumentRevisions retrieves the revision history of a single project document by ID
func (b *BBoltDriver) GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*models.DocumentRevisionHistory, error) {
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	var revisions []*models.DocumentRevisionHistory
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		var docs []types.DefaultDocumentStructure
		if err := col.All(&docs); err != nil {
			return err
		}

		// Filter documents that are revisions
		for _, doc := range docs {
			// Check if this is a revision of the requested document
			if doc.Meta != nil && doc.Meta.Revision {
				if doc.Meta.RootRevisionID == param.DocumentID || doc.ID == param.DocumentID {
					revision := &models.DocumentRevisionHistory{
						ID:         doc.ID,
						RevisionAt: doc.Meta.RevisionAt,
						Status:     doc.Meta.Status,
					}
					revisions = append(revisions, revision)
				}
			}
		}

		return nil
	})

	return revisions, err
}

// GetSingleRawDocumentFromProject retrieves a single raw document from the project
func (b *BBoltDriver) GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	if param.SinglePageData {
		return &types.DefaultDocumentStructure{
			Key:  param.DocumentID,
			ID:   param.DocumentID,
			Type: param.Model.Name,
			Data: map[string]interface{}{},
			Meta: &types.MetaField{
				LastModifiedBy: &types.SystemUser{},
			},
		}, nil
	}

	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	var doc types.DefaultDocumentStructure
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		return col.FindByID(param.DocumentID, &doc)
	})

	if err != nil {
		return nil, errors.New("document not found")
	}

	if !bboltDocPassesQueryHook(b.Conf, param, &doc) {
		return nil, errors.New("document not found")
	}

	return &doc, nil
}

// QueryMultiDocumentOfProject retrieves multiple documents from a project
func (b *BBoltDriver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	var results []*types.DefaultDocumentStructure
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		if err := col.All(&results); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	var filtered []*types.DefaultDocumentStructure
	for _, doc := range results {
		if param.Model != nil && doc.Type != "" && doc.Type != param.Model.Name {
			continue
		}
		if !bboltDocPassesQueryHook(b.Conf, param, doc) {
			continue
		}
		filtered = append(filtered, doc)
	}
	results = filtered

	// Handle payload for non-system requests
	if !param.IsSystemRequest {
		for _, doc := range results {
			utility.HandlePayload(param.Model, doc.Data)
		}
	}

	// TODO: Apply where filter from param.ResolveParams.Args["where"]
	// TODO: Apply limit, skip, sort from param.ResolveParams

	return results, nil
}

// QueryMultiDocumentOfProjectBytes queries multiple documents and returns bytes
func (b *BBoltDriver) QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	var intersect bool
	if param.ResolveParams != nil {
		if val, ok := param.ResolveParams.Args["intersect"].(bool); ok {
			intersect = val
			param.IsIntersectionResult = intersect
		}
	}

	var connection map[string]interface{}
	if param.ResolveParams != nil {
		if val, ok := param.ResolveParams.Args["connection"].(map[string]interface{}); ok && len(val) > 0 {
			connection = val
			if connection["model"] == nil {
				connection["model"] = param.Model.Name
			}
		}
	}

	// Get the documents
	docs, err := b.QueryMultiDocumentOfProject(ctx, param)
	if err != nil {
		return nil, err
	}

	// Convert to bytes
	resp, err := json.Marshal(docs)
	if err != nil {
		return nil, err
	}

	if debug := getEnv("DEBUG", "false"); debug == "true" {
		fmt.Println(fmt.Sprintf("Project : %s | Model : %s | Result : %s", param.ProjectID, param.Model.Name, string(resp)))
	}

	return resp, nil
}

// AddDocumentToProject adds a document to a project
func (b *BBoltDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	// Generate new ObjectId and set it as the Key
	objectID := utility.NewID()
	doc.Key = objectID
	doc.ID = objectID

	row := map[string]interface{}{}
	if err := runDocumentPreInsertHook(b.Conf, ctx, param, row); err != nil {
		return nil, err
	}
	mergeHookRowIntoDocData(doc, row)
	if err := runDocumentPreInsertDocHook(b.Conf, ctx, param, doc); err != nil {
		return nil, err
	}

	err := b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		_, err := col.Save(doc)
		return err
	})

	if err != nil {
		return nil, err
	}

	return doc, nil
}

// UpdateDocumentOfProject updates a document in a project
func (b *BBoltDriver) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	// Update metadata
	doc.Meta.UpdatedAt = utility.GetCurrentTime()
	if doc.Meta.LastModifiedBy == nil {
		doc.Meta.LastModifiedBy = &types.SystemUser{}
	}

	// If created by is null, assign default values
	if doc.Meta.CreatedBy == nil {
		doc.Meta.CreatedAt = utility.GetCurrentTime()
		doc.Meta.CreatedBy = &types.SystemUser{
			ID: param.UserID,
		}
	}

	// Update last modified by
	doc.Meta.LastModifiedBy.ID = param.UserID

	// Check if we need to keep revision
	var keepRevision bool
	switch param.Plan {
	case "pro":
		keepRevision = true
	default:
		// published -> draft
		if doc.Meta.Status == "published" && param.DocPublishStatus == "draft" {
			keepRevision = true
		}
	}

	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)

		if keepRevision {
			// Generate new UUID for the document
			newUUID := utility.NewID()
			doc.ID = newUUID
			doc.Key = newUUID

			doc.Meta.Status = param.DocPublishStatus
			// Create a new copy with new ID
			if _, err := col.Save(doc); err != nil {
				return err
			}

			// Get the old document
			raw, err := b.GetSingleRawDocumentFromProject(ctx, param)
			if err != nil {
				return err
			}
			oldDoc := raw.(*types.DefaultDocumentStructure)

			// Update the old document with revision info
			oldDoc.Meta.RootRevisionID = newUUID
			oldDoc.Meta.Revision = true
			oldDoc.Meta.RevisionAt = utility.GetCurrentTime()

			_, err = col.Save(oldDoc)
			return err
		}

		if doc.Meta.Status == "" {
			doc.Meta.Status = param.DocPublishStatus
		}

		// Check if document exists for single page data
		if param.SinglePageData {
			var existingDoc types.DefaultDocumentStructure
			err := col.FindByID(doc.ID, &existingDoc)
			if err != nil {
				// Document doesn't exist, create it
				_, err = col.Save(doc)
				return err
			}
		}

		// Update or replace the document
		doc.ID = param.DocumentID
		_, err := col.Save(doc)
		return err
	})
}

// DeleteDocumentFromProject deletes a document from a project
func (b *BBoltDriver) DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	if err := b.bboltDeleteAllowed(ctx, param, collectionName); err != nil {
		return err
	}

	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		return col.DeleteStruct(&types.DefaultDocumentStructure{ID: param.DocumentID})
	})
}

// DeleteDocumentsFromProject deletes multiple documents from the project
func (b *BBoltDriver) DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)

		for _, docID := range param.DocumentIDs {
			if err := col.DeleteStruct(&types.DefaultDocumentStructure{ID: docID}); err != nil {
				// Continue even if delete fails
				continue
			}
		}

		return nil
	})
}

// DeleteDocumentRelation deletes all relations or data in pivot tables from the project
func (b *BBoltDriver) DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error {
	// Since ApitoBolt doesn't have ListCollections, we need to check known relation patterns
	// For now, delete from the project-specific relation collection
	relationCollectionName := fmt.Sprintf("p_%s_relation", param.ProjectID)

	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(relationCollectionName)
		var relations []models.EdgeRelation
		if err := col.All(&relations); err != nil {
			return nil // Collection might not exist
		}

		// Delete relations where this document is either from or to
		for _, rel := range relations {
			if rel.FromID == param.DocumentID || rel.ToID == param.DocumentID {
				if err := col.DeleteStruct(&rel); err != nil {
					continue
				}
			}
		}

		return nil
	})
}

// ============================================================================
// COUNTING OPERATIONS
// ============================================================================

// CountDocOfProject counts the documents in the project
func (b *BBoltDriver) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	count, err := b.CountMultiDocumentOfProject(ctx, param, false)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"count": count}, nil
}

// CountDocOfProjectBytes counts the documents and returns bytes
func (b *BBoltDriver) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	result, err := b.CountDocOfProject(ctx, param)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

// CountMultiDocumentOfProject counts documents in a project
func (b *BBoltDriver) CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, previewModel bool) (int, error) {
	collectionName := param.Model.Name

	var count int
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		var docs []types.DefaultDocumentStructure
		if err := col.All(&docs); err != nil {
			return err
		}

		count = len(docs)

		// TODO: Apply where filter from param.ResolveParams

		return nil
	})

	return count, err
}

// ============================================================================
// FIELD OPERATIONS
// ============================================================================

// DropField drops/deletes a field and its data from the project
func (b *BBoltDriver) DropField(ctx context.Context, param *models.CommonSystemParams) error {
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	fieldName := param.FieldInfo.Identifier

	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		var docs []types.DefaultDocumentStructure
		if err := col.All(&docs); err != nil {
			return err
		}

		for _, doc := range docs {
			if doc.Data != nil {
				delete(doc.Data, fieldName)
				if _, err := col.Save(&doc); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

// RenameField renames a field in a model along with its data key
func (b *BBoltDriver) RenameField(ctx context.Context, oldFieldName string, repeatedFieldGroup string, param *models.CommonSystemParams) error {
	collectionName := param.Model.Name
	newFieldName := param.FieldInfo.Identifier

	return b.Store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		var docs []types.DefaultDocumentStructure
		if err := col.All(&docs); err != nil {
			return err
		}

		for _, doc := range docs {
			if doc.Data != nil {
				if value, ok := doc.Data[oldFieldName]; ok {
					doc.Data[newFieldName] = value
					delete(doc.Data, oldFieldName)
					if _, err := col.Save(&doc); err != nil {
						return err
					}
				}
			}
		}

		return nil
	})
}

// ============================================================================
// USER OPERATIONS
// ============================================================================

// GetProjectUser retrieves a user profile by phone, email, and project ID
func (b *BBoltDriver) GetProjectUser(ctx context.Context, phone, email, projectId string) (*types.DefaultDocumentStructure, error) {
	collectionName := fmt.Sprintf("p_%s", projectId)

	var result *types.DefaultDocumentStructure
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		var docs []types.DefaultDocumentStructure
		if err := col.All(&docs); err != nil {
			return err
		}

		// Find user by phone or email
		for _, doc := range docs {
			if doc.Data != nil {
				if phoneVal, ok := doc.Data["phone"].(string); ok && phoneVal == phone {
					result = &doc
					return nil
				}
				if emailVal, ok := doc.Data["email"].(string); ok && emailVal == email {
					result = &doc
					return nil
				}
			}
		}

		return errors.New("user not found")
	})

	if err != nil {
		return nil, nil // User not found
	}

	return result, nil
}

// GetLoggedInProjectUser retrieves the logged-in user profile for the project
func (b *BBoltDriver) GetLoggedInProjectUser(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	if param.DocumentID == "" {
		return nil, errors.New("user ID is required")
	}

	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	var result types.DefaultDocumentStructure
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		return col.FindByID(param.DocumentID, &result)
	})

	if err != nil {
		return nil, nil // User not found
	}

	return &result, nil
}

// GetProjectUsers retrieves metadata for multiple users in the project
func (b *BBoltDriver) GetProjectUsers(ctx context.Context, projectId string, keys []string) (map[string]*types.DefaultDocumentStructure, error) {
	if len(keys) == 0 {
		return make(map[string]*types.DefaultDocumentStructure), nil
	}

	collectionName := fmt.Sprintf("p_%s", projectId)
	result := make(map[string]*types.DefaultDocumentStructure)

	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)

		for _, key := range keys {
			var user types.DefaultDocumentStructure
			if err := col.FindByID(key, &user); err == nil {
				result[user.ID] = &user
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to query project users: %w", err)
	}

	return result, nil
}

// ============================================================================
// RELATION DATA OPERATIONS
// ============================================================================

// GetAllRelationDocumentsOfSingleDocument retrieves all relation data of a single document by ID
func (b *BBoltDriver) GetAllRelationDocumentsOfSingleDocument(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error) {
	// This would need to query all relation collections
	// For now, return empty result
	return []interface{}{}, nil
}

// AddTeamMetaInfo adds metadata information for a team in the project
func (b *BBoltDriver) AddTeamMetaInfo(ctx context.Context, users []*models.SystemUser) ([]*models.SystemUser, error) {
	// Get user IDs
	var userIds []string
	for _, u := range users {
		userIds = append(userIds, u.ID)
	}

	// Query users collection
	usersMap := make(map[string]*types.DefaultDocumentStructure)
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection("users")
		for _, userID := range userIds {
			var user types.DefaultDocumentStructure
			if err := col.FindByID(userID, &user); err == nil {
				usersMap[userID] = &user
			}
		}
		return nil
	})

	if err != nil {
		return users, nil
	}

	// Add meta info to team members
	for _, u := range users {
		if userDoc, ok := usersMap[u.ID]; ok {
			if userDoc.Data != nil {
				if firstName, ok := userDoc.Data["first_name"].(string); ok {
					u.FirstName = firstName
				}
				if lastName, ok := userDoc.Data["last_name"].(string); ok {
					u.LastName = lastName
				}
				if email, ok := userDoc.Data["email"].(string); ok {
					u.Email = email
				}
			}
		}
	}

	return users, nil
}

// RelationshipDataLoader loads relationship data for the project
func (b *BBoltDriver) RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) (interface{}, error) {
	modelName, ok := connection["model"].(string)
	if !ok {
		return nil, errors.New("model not specified in connection")
	}

	relationField, ok := connection["field"].(string)
	if !ok {
		return nil, errors.New("field not specified in connection")
	}

	// Get the target collection name
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	// Get relation IDs for the source document
	relationCollectionName := fmt.Sprintf("relation_%s_%s", param.Model.Name, modelName)

	var targetIDs []string
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		relCol := tx.Collection(relationCollectionName)
		var relations []models.EdgeRelation
		if err := relCol.All(&relations); err != nil {
			return err
		}

		for _, relation := range relations {
			if relation.KnownAs == relationField {
				targetIDs = append(targetIDs, relation.ToID)
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(targetIDs) == 0 {
		return []interface{}{}, nil
	}

	// Query target documents
	var results []interface{}
	err = b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)

		for _, targetID := range targetIDs {
			var doc types.DefaultDocumentStructure
			if err := col.FindByID(targetID, &doc); err == nil {
				results = append(results, &doc)
			}
		}

		return nil
	})

	return results, err
}

// RelationshipDataLoaderBytes loads relationship data for the project and returns it as bytes
func (b *BBoltDriver) RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) ([]byte, error) {
	data, err := b.RelationshipDataLoader(ctx, param, connection)
	if err != nil {
		return nil, err
	}
	return json.Marshal(data)
}

// ============================================================================
// AGGREGATION OPERATIONS
// ============================================================================

// AggregateDocOfProject aggregates the documents in the project
func (b *BBoltDriver) AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	// Simplified aggregation - return count for now
	// TODO: Implement full aggregation pipeline
	count, err := b.CountMultiDocumentOfProject(ctx, param, false)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"count": count,
	}, nil
}

// AggregateDocOfProjectBytes aggregates the documents and returns bytes
func (b *BBoltDriver) AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	result, err := b.AggregateDocOfProject(ctx, param)
	if err != nil {
		return nil, err
	}
	return json.Marshal(result)
}

// ============================================================================
// ADDITIONAL INTERFACE METHODS
// ============================================================================

// SearchFunctions lists all the cloud functions for a given project
func (b *BBoltDriver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	return &models.SearchResponse[models.ApitoFunction]{}, nil
}

// SearchWebHooks lists all the webhooks for a given project
func (b *BBoltDriver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	return &models.SearchResponse[models.Webhook]{}, nil
}

// GetWebHook retrieves a webhook by project ID and hook ID
func (b *BBoltDriver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	return nil, nil
}

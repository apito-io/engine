package mongo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaults string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaults
}

// DeleteProject implements the deletion of a project
func (m *MongoDriver) DeleteProject(ctx context.Context, projectId string) error {
	// Define collection names
	projectCollection := fmt.Sprintf("p_%s", projectId)
	projectMediaCollection := fmt.Sprintf("%s_media", projectCollection)
	projectEdgeCollection := fmt.Sprintf("%s_relation", projectCollection)

	// Start a session for transaction
	session, err := m.Client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	// Execute operations in a transaction
	_, err = session.WithTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		// Drop the main project collection
		err := m.Database.Collection(projectCollection).Drop(sessCtx)
		if err != nil {
			return nil, err
		}

		// Drop the media collection
		err = m.Database.Collection(projectMediaCollection).Drop(sessCtx)
		if err != nil {
			return nil, err
		}

		// Drop the relation collection
		err = m.Database.Collection(projectEdgeCollection).Drop(sessCtx)
		if err != nil {
			return nil, err
		}

		return nil, nil
	})

	return err
}

// CheckCollectionExists checks if a collection exists in the project
func (m *MongoDriver) CheckCollectionExists(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) (bool, error) {
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		if isRelationCollection {
			collectionName = fmt.Sprintf("p_%s_relation", param.ProjectID)
		} else {
			collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
		}
	} else {
		collectionName = param.Model.Name
		if isRelationCollection {
			collectionName = "relation_" + collectionName
		}
	}

	collections, err := m.Database.ListCollectionNames(ctx, bson.M{"name": collectionName})
	if err != nil {
		return false, err
	}

	for _, name := range collections {
		if name == collectionName {
			return true, nil
		}
	}
	return false, nil
}

// AddCollection adds a new collection to the project
func (m *MongoDriver) AddCollection(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) error {
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		if isRelationCollection {
			collectionName = fmt.Sprintf("p_%s_relation", param.ProjectID)
		} else {
			collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
		}
	} else {
		collectionName = param.Model.Name
		if isRelationCollection {
			collectionName = "relation_" + collectionName
		}
	}

	// Create the collection
	err := m.Database.CreateCollection(ctx, collectionName)
	if err != nil {
		return err
	}

	// Create indexes
	collection := m.Database.Collection(collectionName)

	// Create TTL index for expire_at
	ttlIndex := mongo.IndexModel{
		Keys:    bson.D{{"expire_at", 1}},
		Options: options.Index().SetName("doc_expire_at").SetExpireAfterSeconds(1),
	}
	_, err = collection.Indexes().CreateOne(ctx, ttlIndex)
	if err != nil {
		return err
	}

	if param.ProjectType == models.ProjectType_SaaS {
		// Create tenant_id index for SaaS projects
		tenantIndex := mongo.IndexModel{
			Keys:    bson.D{{"tenant_id", 1}},
			Options: options.Index().SetName("tenant_id"),
		}
		_, err = collection.Indexes().CreateOne(ctx, tenantIndex)
		if err != nil {
			return err
		}
	}

	if isRelationCollection {
		// Create indexes for relation collections
		fromIndex := mongo.IndexModel{
			Keys:    bson.D{{"from", 1}},
			Options: options.Index().SetName("from_relation"),
		}
		_, err = collection.Indexes().CreateOne(ctx, fromIndex)
		if err != nil {
			return err
		}

		toIndex := mongo.IndexModel{
			Keys:    bson.D{{"to", 1}},
			Options: options.Index().SetName("to_relation"),
		}
		_, err = collection.Indexes().CreateOne(ctx, toIndex)
		if err != nil {
			return err
		}

		knownAsIndex := mongo.IndexModel{
			Keys:    bson.D{{"known_as", 1}},
			Options: options.Index().SetName("known_as_relation"),
		}
		_, err = collection.Indexes().CreateOne(ctx, knownAsIndex)
		if err != nil {
			return err
		}

		fromIdIndex := mongo.IndexModel{
			Keys:    bson.D{{"from_id", 1}},
			Options: options.Index().SetName("from_id_relation"),
		}
		_, err = collection.Indexes().CreateOne(ctx, fromIdIndex)
		if err != nil {
			return err
		}

		toIdIndex := mongo.IndexModel{
			Keys:    bson.D{{"to_id", 1}},
			Options: options.Index().SetName("to_id_relation"),
		}
		_, err = collection.Indexes().CreateOne(ctx, toIdIndex)
		if err != nil {
			return err
		}
	} else {
		// Create indexes for regular collections
		typeIndex := mongo.IndexModel{
			Keys:    bson.D{{"type", 1}},
			Options: options.Index().SetName("doc_type"),
		}
		_, err = collection.Indexes().CreateOne(ctx, typeIndex)
		if err != nil {
			return err
		}

		statusIndex := mongo.IndexModel{
			Keys:    bson.D{{"meta.status", 1}},
			Options: options.Index().SetName("doc_publish_status"),
		}
		_, err = collection.Indexes().CreateOne(ctx, statusIndex)
		if err != nil {
			return err
		}
	}

	return nil
}

// TransferProject transfers a project from one user to another
func (m *MongoDriver) TransferProject(ctx context.Context, userId, from, to string) error {
	// Transfer main documents
	err := m.transferDocs(ctx, userId, from, to)
	if err != nil {
		return err
	}

	// Transfer relation documents
	err = m.transferRelationDocs(ctx, from, to)
	if err != nil {
		return err
	}

	// Transfer media documents
	err = m.transferMediaDocs(ctx, from, to)
	if err != nil {
		return err
	}

	return nil
}

// transferDocs transfers documents from one project to another
func (m *MongoDriver) transferDocs(ctx context.Context, userId, from, to string) error {
	// Source collection name
	fromCollectionName := "p_" + from

	// Get all documents from source collection
	cursor, err := m.Database.Collection(fromCollectionName).Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	// Decode all documents
	var docs []*types.DefaultDocumentStructure
	if err = cursor.All(ctx, &docs); err != nil {
		return err
	}

	// Target collection name
	toCollectionName := "p_" + to
	toCollection := m.Database.Collection(toCollectionName)

	// Insert documents into target collection
	for _, doc := range docs {
		// Update metadata
		doc.Meta.CreatedAt = utility.GetCurrentTime()
		doc.Meta.UpdatedAt = utility.GetCurrentTime()
		doc.Meta.CreatedBy = &types.SystemUser{ID: userId}
		doc.Meta.LastModifiedBy = &types.SystemUser{ID: userId}

		_, err = toCollection.InsertOne(ctx, doc)
		if err != nil {
			return err
		}
	}

	return nil
}

// transferMediaDocs transfers media documents from one project to another
func (m *MongoDriver) transferMediaDocs(ctx context.Context, from, to string) error {
	// Source collection name
	fromCollectionName := "p_" + from + "_media"

	// Get all documents from source collection
	cursor, err := m.Database.Collection(fromCollectionName).Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	// Decode all documents
	var docs []*models.FileDetails
	if err = cursor.All(ctx, &docs); err != nil {
		return err
	}

	// Target collection name
	toCollectionName := "p_" + to + "_media"
	toCollection := m.Database.Collection(toCollectionName)

	// Insert documents into target collection
	for _, doc := range docs {
		// Update the URL path
		doc.URL = strings.Replace(doc.URL, from, to, -1)
		if doc.UploadParam != nil {
			doc.UploadParam.ProjectID = to
		}

		// Update metadata
		doc.CreatedAt = utility.GetCurrentTime()

		_, err = toCollection.InsertOne(ctx, doc)
		if err != nil {
			return err
		}
	}

	return nil
}

// transferRelationDocs transfers relation documents from one project to another
func (m *MongoDriver) transferRelationDocs(ctx context.Context, from, to string) error {
	// Source collection name
	fromCollectionName := "p_" + from + "_relation"

	// Get all documents from source collection
	cursor, err := m.Database.Collection(fromCollectionName).Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	// Decode all documents
	var docs []models.EdgeRelation
	if err = cursor.All(ctx, &docs); err != nil {
		return err
	}

	// Target collection name
	toCollectionName := "p_" + to + "_relation"
	toCollection := m.Database.Collection(toCollectionName)

	// Insert documents into target collection
	for _, doc := range docs {
		// Update the from and to relations
		doc.XFrom = strings.Replace(doc.XFrom, from, to, -1)
		doc.XTo = strings.Replace(doc.XTo, from, to, -1)

		// Update metadata
		doc.CreatedAt = utility.GetCurrentTime()

		_, err = toCollection.InsertOne(ctx, doc)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetProject retrieves a project by ID
func (m *MongoDriver) GetProject(ctx context.Context, id string) (*models.Project, error) {
	var project models.Project

	// Query the projects collection
	collection := m.Database.Collection("projects")
	filter := bson.M{"_id": id}
	err := collection.FindOne(ctx, filter).Decode(&project)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("no Project Found")
		}
		return nil, err
	}

	if project.ID == "" {
		return nil, errors.New("no Project Found")
	}

	return &project, nil
}

// GetSingleProjectDocument gets a single document from a project
func (m *MongoDriver) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	// Handle intersection results
	if param.ResolveParams != nil {
		if val, ok := param.ResolveParams.Args["intersect"].(bool); ok {
			param.IsIntersectionResult = val
		}
	}

	// Handle tenant model
	if param.TenantModel == param.Model.Name {
		param.TenantModel = ""
		param.TenantID = ""
	}

	// Build filter
	filter := bson.M{"_id": param.DocumentID}
	if param.TenantID != "" {
		filter["tenant_id"] = param.TenantID
	}

	// Get collection name
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		if param.SinglePageData {
			collectionName = fmt.Sprintf("p_%s_single_docs", param.ProjectID)
		} else {
			collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
		}
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	// Query the document
	collection := m.Database.Collection(collectionName)
	var doc types.DefaultDocumentStructure
	err := collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("no result found with this request id")
		}
		return nil, err
	}

	// Handle payload for non-system requests
	if !param.IsSystemRequest {
		utility.HandlePayload(param.Model, doc.Data)
	}

	return &doc, nil
}

// DeleteDocumentFromProject deletes a document from a project
func (m *MongoDriver) DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		if param.SinglePageData {
			collectionName = fmt.Sprintf("p_%s_single_docs", param.ProjectID)
		} else {
			collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
		}
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	collection := m.Database.Collection(collectionName)
	filter := bson.M{"_id": param.DocumentID}

	if param.TenantID != "" {
		filter["tenant_id"] = param.TenantID
	}

	_, err := collection.DeleteOne(ctx, filter)
	return err
}

// QueryMultiDocumentOfProject retrieves multiple documents from a project
func (m *MongoDriver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		if param.SinglePageData {
			collectionName = fmt.Sprintf("p_%s_single_docs", param.ProjectID)
		} else {
			collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
		}
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	collection := m.Database.Collection(collectionName)
	findOpts := options.Find()

	// Build filter
	filter := bson.M{}
	if param.TenantID != "" {
		filter["tenant_id"] = param.TenantID
	}

	// Apply pagination if provided in resolve params
	if param.ResolveParams != nil {
		if skip, ok := param.ResolveParams.Args["skip"].(int); ok {
			findOpts.SetSkip(int64(skip))
		}
		if limit, ok := param.ResolveParams.Args["limit"].(int); ok {
			findOpts.SetLimit(int64(limit))
		}

		// Apply sorting if provided
		if sort, ok := param.ResolveParams.Args["sort"].(map[string]interface{}); ok {
			sortDoc := bson.D{}
			for field, direction := range sort {
				value := 1
				if direction == "desc" {
					value = -1
				}
				sortDoc = append(sortDoc, bson.E{Key: field, Value: value})
			}
			findOpts.SetSort(sortDoc)
		}

		// Apply where filter if provided
		if where, ok := param.ResolveParams.Args["where"].(map[string]interface{}); ok {
			for k, v := range where {
				filter[k] = v
			}
		}
	}

	cursor, err := collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []*types.DefaultDocumentStructure
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// Handle payload for non-system requests
	if !param.IsSystemRequest {
		for _, doc := range results {
			utility.HandlePayload(param.Model, doc.Data)
		}
	}

	return results, nil
}

// CountMultiDocumentOfProject counts documents in a project
func (m *MongoDriver) CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, previewModel bool) (int, error) {
	collection := m.Database.Collection(param.Model.Name)
	filter := bson.M{}

	// Apply filter if provided in resolve params
	if param.ResolveParams != nil {
		if where, ok := param.ResolveParams.Args["where"].(map[string]interface{}); ok {
			filter = where
		}
	}

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}

// Unimplemented interface methods - these will return "not implemented" errors until implemented

func (m *MongoDriver) AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error) {
	if model.SinglePage {
		uid := uuid.New()
		model.SinglePageUUID = uid.String()
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
	err := m.Database.CreateCollection(ctx, collectionName)
	if err != nil {
		return nil, err
	}

	collection := m.Database.Collection(collectionName)
	indexOpts := options.Index().SetName("id_index")
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "_id", Value: 1}},
		Options: indexOpts,
	}

	_, err = collection.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		return nil, err
	}

	return project.Schema, nil
}

// AddFieldToModel adds a new field to an existing model in the project.
func (m *MongoDriver) AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parent_field string) (*models.ModelType, error) {
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
		// Add new field
		targetModel.Fields = append(targetModel.Fields, param.FieldInfo)
	} else {
		// Adding to a nested/repeated group
		depth := m.searchAndAppend(targetModel.Fields, isUpdate, parent_field, param.FieldInfo, 0)
		if depth == 0 {
			return nil, errors.New("parent field not found")
		}
	}

	return targetModel, nil
}

func (m *MongoDriver) RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error {
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
	oldCollection := m.Database.Collection(modelName)
	newCollection := m.Database.Collection(newName)

	// Get all documents from old collection
	cursor, err := oldCollection.Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	// Create new collection
	err = m.Database.CreateCollection(ctx, newName)
	if err != nil {
		return err
	}

	// Transfer documents to new collection
	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return err
		}
		_, err = newCollection.InsertOne(ctx, doc)
		if err != nil {
			return err
		}
	}

	// Drop old collection
	err = oldCollection.Drop(ctx)
	if err != nil {
		return err
	}

	// Update model name in schema
	modelToRename.Name = newName

	return nil
}

func (m *MongoDriver) ConvertModel(ctx context.Context, project *models.Project, modelName string) error {
	// Find the model to convert
	var modelToConvert *models.ModelType
	for _, model := range project.Schema.Models {
		if model.Name == modelName {
			modelToConvert = model
			break
		}
	}

	if modelToConvert == nil {
		return errors.New("model not found")
	}

	// Create a new collection with the converted name
	newCollectionName := "converted_" + modelName
	err := m.Database.CreateCollection(ctx, newCollectionName)
	if err != nil {
		return err
	}

	// Get the original collection
	originalCollection := m.Database.Collection(modelName)
	newCollection := m.Database.Collection(newCollectionName)

	// Copy documents to new collection
	cursor, err := originalCollection.Find(ctx, bson.M{})
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return err
		}
		_, err = newCollection.InsertOne(ctx, doc)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *MongoDriver) DropModel(ctx context.Context, project *models.Project, modelName string) error {
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
	collection := m.Database.Collection(modelName)
	return collection.Drop(ctx)
}

func (m *MongoDriver) CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parent_field string) error {
	collection := m.Database.Collection(param.Model.Name)

	// Create index options
	indexOpts := options.Index().SetName(fieldName + "_index")

	// Create index model
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: fieldName, Value: 1}},
		Options: indexOpts,
	}

	_, err := collection.Indexes().CreateOne(ctx, indexModel)
	return err
}

func (m *MongoDriver) DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error {
	collection := m.Database.Collection(param.Model.Name)
	err := collection.Indexes().DropOne(ctx, indexName)
	return err
}

func (m *MongoDriver) AddRelationFields(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error {
	// Create relation collection name
	relationCollectionName := fmt.Sprintf("relation_%s_%s", from.Model, to.Model)

	// Create relation collection
	err := m.Database.CreateCollection(ctx, relationCollectionName)
	if err != nil {
		return err
	}

	// Create indexes for the relation collection
	collection := m.Database.Collection(relationCollectionName)

	// Create index for from field
	fromIndexOpts := options.Index().SetName("from_index")
	fromIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "from", Value: 1}},
		Options: fromIndexOpts,
	}
	_, err = collection.Indexes().CreateOne(ctx, fromIndex)
	if err != nil {
		return err
	}

	// Create index for to field
	toIndexOpts := options.Index().SetName("to_index")
	toIndex := mongo.IndexModel{
		Keys:    bson.D{{Key: "to", Value: 1}},
		Options: toIndexOpts,
	}
	_, err = collection.Indexes().CreateOne(ctx, toIndex)
	return err
}

func (m *MongoDriver) DeleteRelationDocuments(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error {
	relationCollectionName := fmt.Sprintf("relation_%s_%s", from.Model, to.Model)
	collection := m.Database.Collection(relationCollectionName)

	// Delete all documents in the relation collection
	_, err := collection.DeleteMany(ctx, bson.M{})
	return err
}

func (m *MongoDriver) GetRelationDocument(ctx context.Context, param *models.ConnectDisconnectParam) (*models.EdgeRelation, error) {
	relationCollectionName := fmt.Sprintf("relation_%s_%s", param.ForwardConnectionType.Model, param.BackwardConnectionType.Model)
	collection := m.Database.Collection(relationCollectionName)

	filter := bson.M{
		"from_id": param.ForwardConnectionID,
		"to_id":   param.ActionIDs[0],
	}

	var result models.EdgeRelation
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("relation not found")
		}
		return nil, err
	}

	return &result, nil
}

func (m *MongoDriver) CreateRelation(ctx context.Context, projectId string, relation *models.EdgeRelation) error {
	relationCollectionName := fmt.Sprintf("relation_%s_%s", relation.From, relation.To)
	collection := m.Database.Collection(relationCollectionName)

	_, err := collection.InsertOne(ctx, relation)
	return err
}

func (m *MongoDriver) DeleteRelation(ctx context.Context, param *models.ConnectDisconnectParam, id string) error {
	relationCollectionName := fmt.Sprintf("relation_%s_%s", param.ForwardConnectionType.Model, param.BackwardConnectionType.Model)
	collection := m.Database.Collection(relationCollectionName)

	filter := bson.M{
		"_id": id,
	}

	_, err := collection.DeleteOne(ctx, filter)
	return err
}

// NewInsertableRelations retrieves new insertable relations in the project.
func (m *MongoDriver) NewInsertableRelations(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	relationCollectionName := fmt.Sprintf("relation_%s_%s", param.ForwardConnectionType.Model, param.BackwardConnectionType.Model)
	collection := m.Database.Collection(relationCollectionName)

	// Find existing relations for this document
	filter := bson.M{
		"from_id": param.ForwardConnectionID,
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Collect existing relation IDs
	existingRelations := make(map[string]bool)
	for cursor.Next(ctx) {
		var relation models.EdgeRelation
		if err := cursor.Decode(&relation); err != nil {
			return nil, err
		}
		existingRelations[relation.ToID] = true
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

// CheckOneToOneRelationExists checks if a one-to-one relation exists in the project.
func (m *MongoDriver) CheckOneToOneRelationExists(ctx context.Context, param *models.ConnectDisconnectParam) (bool, error) {
	relationCollectionName := fmt.Sprintf("relation_%s_%s", param.ForwardConnectionType.Model, param.BackwardConnectionType.Model)
	collection := m.Database.Collection(relationCollectionName)

	// Check if there's already a relation from this document
	filter := bson.M{
		"from_id": param.ForwardConnectionID,
	}

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// GetRelationIds retrieves the IDs of every document related to a document.
func (m *MongoDriver) GetRelationIds(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	relationCollectionName := fmt.Sprintf("relation_%s_%s", param.ForwardConnectionType.Model, param.BackwardConnectionType.Model)
	collection := m.Database.Collection(relationCollectionName)

	filter := bson.M{
		"from_id": param.ForwardConnectionID,
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var relationIds []string
	for cursor.Next(ctx) {
		var relation models.EdgeRelation
		if err := cursor.Decode(&relation); err != nil {
			return nil, err
		}
		relationIds = append(relationIds, relation.ToID)
	}

	return relationIds, nil
}

// ConnectBuilder connects a builder to the project.
func (m *MongoDriver) ConnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
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
		newRelations, err := m.NewInsertableRelations(ctx, connParam)
		if err != nil {
			return err
		}
		if len(newRelations) == 0 {
			continue
		}

		// add a new relation for each action ID
		for _, id := range connParam.ActionIDs {
			var fromID, toID string
			if param.ProjectType == models.ProjectType_SaaS {
				if param.TenantID == "" {
					return errors.New("tenant id is required if project type is SaaS")
				}

				switch connParam.ConnectionType {
				case "forward":
					fromID = connParam.ForwardConnectionID
					toID = id
				case "backward":
					fromID = id
					toID = connParam.ForwardConnectionID
				default:
					return errors.New("invalid connection type")
				}
			} else {
				fromID = connParam.ForwardConnectionID
				toID = id
			}

			// insert relation
			relation := &models.EdgeRelation{
				Key:       primitive.NewObjectID().Hex(),
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
			if param.TenantID != "" {
				relation.TenantID = param.TenantID
			}

			err := m.CreateRelation(ctx, param.ProjectID, relation)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// DisconnectBuilder disconnects a builder from the project.
func (m *MongoDriver) DisconnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
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

				// find out the relation document
				relationDoc, err := m.GetRelationDocument(ctx, connParam)
				if err != nil {
					return err
				}

				// remove relation
				err = m.DeleteRelation(ctx, connParam, relationDoc.Key)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// GetAllRelationDocumentsOfSingleDocument retrieves all relation data of a single document by ID.
func (m *MongoDriver) GetAllRelationDocumentsOfSingleDocument(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error) {
	var relations []interface{}

	// Since we don't have direct access to the project schema, we'll look for all possible relation collections
	// This is a simplified approach - in practice, you'd want to pass the schema or get it from a cache
	collections, err := m.Database.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	for _, collectionName := range collections {
		// Look for relation collections (they typically have "relation" in the name)
		if strings.Contains(collectionName, "relation") {
			collection := m.Database.Collection(collectionName)

			filter := bson.M{
				"from_id": from,
			}

			cursor, err := collection.Find(ctx, filter)
			if err != nil {
				continue // Skip this collection if there's an error
			}

			for cursor.Next(ctx) {
				var relation models.EdgeRelation
				if err := cursor.Decode(&relation); err != nil {
					continue
				}
				relations = append(relations, relation)
			}
			cursor.Close(ctx)
		}
	}

	return relations, nil
}

// GetSingleProjectDocumentBytes retrieves a single project document by ID as bytes.
func (m *MongoDriver) GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	doc, err := m.GetSingleProjectDocument(ctx, param)
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

// GetSingleProjectDocumentRevisions retrieves the revision history of a single project document by ID.
func (m *MongoDriver) GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*models.DocumentRevisionHistory, error) {
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		if param.SinglePageData {
			collectionName = fmt.Sprintf("p_%s_single_docs", param.ProjectID)
		} else {
			collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
		}
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	collection := m.Database.Collection(collectionName)

	// Find documents that are revisions of the specified document
	filter := bson.M{
		"$or": []bson.M{
			{"meta.root_revision_id": param.DocumentID},
			{"_id": param.DocumentID},
		},
		"meta.revision": true,
	}

	if param.TenantID != "" {
		filter["tenant_id"] = param.TenantID
	}

	// Sort by revision date descending
	findOpts := options.Find().SetSort(bson.D{{"meta.revision_at", -1}})

	cursor, err := collection.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var revisions []*models.DocumentRevisionHistory
	for cursor.Next(ctx) {
		var doc types.DefaultDocumentStructure
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}

		revision := &models.DocumentRevisionHistory{
			ID:         doc.ID,
			RevisionAt: doc.Meta.RevisionAt,
			Status:     doc.Meta.Status,
		}
		revisions = append(revisions, revision)
	}

	return revisions, nil
}

func (m *MongoDriver) GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
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

	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	collection := m.Database.Collection(collectionName)
	filter := bson.M{"_id": param.DocumentID}

	var doc types.DefaultDocumentStructure
	err := collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("document not found")
		}
		return nil, err
	}

	return &doc, nil
}

func (m *MongoDriver) QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	var intersect bool
	if val, ok := param.ResolveParams.Args["intersect"].(bool); ok {
		intersect = val
		param.IsIntersectionResult = intersect
	}

	var connection map[string]interface{}
	if val, ok := param.ResolveParams.Args["connection"].(map[string]interface{}); ok && len(val) > 0 {
		connection = val
		if connection["model"] == nil {
			connection["model"] = param.Model.Name
		}
	}

	// Get the documents
	docs, err := m.QueryMultiDocumentOfProject(ctx, param)
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

func (m *MongoDriver) AddTeamMetaInfo(ctx context.Context, users []*models.SystemUser) ([]*models.SystemUser, error) {
	// Get user IDs
	var userIds []string
	for _, u := range users {
		userIds = append(userIds, u.ID)
	}

	// Query users collection
	collection := m.Database.Collection("users")
	filter := bson.M{"_id": bson.M{"$in": userIds}}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	// Map of user ID to user for quick lookup
	userMap := make(map[string]*models.SystemUser)
	for _, u := range users {
		userMap[u.ID] = u
	}

	// Process results
	var teams []*models.SystemUser
	for cursor.Next(ctx) {
		var team models.SystemUser
		if err := cursor.Decode(&team); err != nil {
			return nil, err
		}
		if user, exists := userMap[team.ID]; exists {
			teams = append(teams, user)
		}
	}

	return teams, nil
}

func (m *MongoDriver) DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		if param.SinglePageData {
			collectionName = fmt.Sprintf("p_%s_single_docs", param.ProjectID)
		} else {
			collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
		}
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	collection := m.Database.Collection(collectionName)
	filter := bson.M{"_id": bson.M{"$in": param.DocumentIDs}}

	if param.TenantID != "" {
		filter["tenant_id"] = param.TenantID
	}

	_, err := collection.DeleteMany(ctx, filter)
	return err
}

// DeleteDocumentRelation deletes all relations or data in pivot tables from the project.
func (m *MongoDriver) DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error {
	// Get all collection names and find relation collections
	collections, err := m.Database.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return err
	}

	for _, collectionName := range collections {
		// Look for relation collections
		if strings.Contains(collectionName, "relation") {
			collection := m.Database.Collection(collectionName)

			// Delete relations where this document is either from or to
			filter := bson.M{
				"$or": []bson.M{
					{"from_id": param.DocumentID},
					{"to_id": param.DocumentID},
				},
			}

			_, err := collection.DeleteMany(ctx, filter)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// RelationshipDataLoader loads relationship data for the project.
func (m *MongoDriver) RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) (interface{}, error) {
	modelName, ok := connection["model"].(string)
	if !ok {
		return nil, errors.New("model not specified in connection")
	}

	relationField, ok := connection["field"].(string)
	if !ok {
		return nil, errors.New("field not specified in connection")
	}

	// Get the target collection name
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, modelName)
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	// Get relation IDs for the source document
	relationCollectionName := fmt.Sprintf("relation_%s_%s", param.Model.Name, modelName)
	relationCollection := m.Database.Collection(relationCollectionName)

	relationFilter := bson.M{
		"from_id":  param.DocumentID,
		"known_as": relationField,
	}

	relationCursor, err := relationCollection.Find(ctx, relationFilter)
	if err != nil {
		return nil, err
	}
	defer relationCursor.Close(ctx)

	var targetIds []string
	for relationCursor.Next(ctx) {
		var relation models.EdgeRelation
		if err := relationCursor.Decode(&relation); err != nil {
			return nil, err
		}
		targetIds = append(targetIds, relation.ToID)
	}

	if len(targetIds) == 0 {
		return []interface{}{}, nil
	}

	// Get the actual documents
	targetCollection := m.Database.Collection(collectionName)
	filter := bson.M{
		"_id": bson.M{"$in": targetIds},
	}

	if param.TenantID != "" {
		filter["tenant_id"] = param.TenantID
	}

	cursor, err := targetCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []interface{}
	for cursor.Next(ctx) {
		var doc types.DefaultDocumentStructure
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		results = append(results, &doc)
	}

	return results, nil
}

// RelationshipDataLoaderBytes loads relationship data for the project and returns it as bytes.
func (m *MongoDriver) RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) ([]byte, error) {
	data, err := m.RelationshipDataLoader(ctx, param, connection)
	if err != nil {
		return nil, err
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	if debug := getEnv("DEBUG", "false"); debug == "true" {
		fmt.Println(fmt.Sprintf("Project : %s | Model : %s | Connection : %v | Result : %s", param.ProjectID, param.Model.Name, connection, string(bytes)))
	}

	return bytes, nil
}

func (m *MongoDriver) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		if param.SinglePageData {
			collectionName = fmt.Sprintf("p_%s_single_docs", param.ProjectID)
		} else {
			collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
		}
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	collection := m.Database.Collection(collectionName)
	filter := bson.M{}

	if param.TenantID != "" {
		filter["tenant_id"] = param.TenantID
	}

	if param.ResolveParams != nil {
		if where, ok := param.ResolveParams.Args["where"].(map[string]interface{}); ok {
			for k, v := range where {
				filter[k] = v
			}
		}
	}

	count, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total": count,
	}, nil
}

func (m *MongoDriver) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	// Get the count
	count, err := m.CountDocOfProject(ctx, param)
	if err != nil {
		return nil, err
	}

	// Convert to bytes
	resp, err := json.Marshal(count)
	if err != nil {
		return nil, err
	}

	if debug := getEnv("DEBUG", "false"); debug == "true" {
		fmt.Println(fmt.Sprintf("Project : %s | Model : %s | Result : %s", param.ProjectID, param.Model.Name, string(resp)))
	}

	return resp, nil
}

func (m *MongoDriver) AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		if param.SinglePageData {
			collectionName = fmt.Sprintf("p_%s_single_docs", param.ProjectID)
		} else {
			collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
		}
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	collection := m.Database.Collection(collectionName)
	pipeline := mongo.Pipeline{}

	// Add match stage for tenant ID
	if param.TenantID != "" {
		pipeline = append(pipeline, bson.D{{"$match", bson.M{"tenant_id": param.TenantID}}})
	}

	// Add match stage if filter exists
	if param.ResolveParams != nil {
		if where, ok := param.ResolveParams.Args["where"].(map[string]interface{}); ok {
			pipeline = append(pipeline, bson.D{{"$match", where}})
		}
	}

	// Add group stage
	pipeline = append(pipeline, bson.D{
		{"$group", bson.D{
			{"_id", nil},
			{"count", bson.D{{"$sum", 1}}},
		}},
	})

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	if err = cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return map[string]interface{}{"count": 0}, nil
	}

	return results[0], nil
}

func (m *MongoDriver) AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	// Get the aggregation result
	data, err := m.AggregateDocOfProject(ctx, param)
	if err != nil {
		return nil, err
	}

	// Convert to bytes
	resp, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	if debug := getEnv("DEBUG", "false"); debug == "true" {
		fmt.Println(fmt.Sprintf("Project : %s | Model : %s | Result : %s", param.ProjectID, param.Model.Name, string(resp)))
	}

	return resp, nil
}

func (m *MongoDriver) DropField(ctx context.Context, param *models.CommonSystemParams) error {
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		if param.SinglePageData {
			collectionName = fmt.Sprintf("p_%s_single_docs", param.ProjectID)
		} else {
			collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
		}
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	collection := m.Database.Collection(collectionName)

	// Build filter for tenant ID if needed
	filter := bson.M{}
	if param.TenantID != "" {
		filter["tenant_id"] = param.TenantID
	}

	// Update all documents to unset the field
	update := bson.M{
		"$unset": bson.M{
			param.FieldInfo.Identifier: "",
		},
	}

	_, err := collection.UpdateMany(ctx, filter, update)
	return err
}

func (m *MongoDriver) RenameField(ctx context.Context, oldFieldName string, repeatedFieldGroup string, param *models.CommonSystemParams) error {
	collection := m.Database.Collection(param.Model.Name)

	// Update all documents to rename the field
	update := bson.M{
		"$rename": bson.M{
			oldFieldName: param.FieldInfo.Identifier,
		},
	}

	_, err := collection.UpdateMany(ctx, bson.M{}, update)
	return err
}

// Additional methods needed to satisfy the interface

func (m *MongoDriver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	return nil, errors.New("method not implemented")
}

func (m *MongoDriver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	return nil, errors.New("method not implemented")
}

func (m *MongoDriver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	return nil, errors.New("method not implemented")
}

// Helper method to search and append fields to a model
func (m *MongoDriver) searchAndAppend(fields []*models.FieldInfo, isUpdate bool, parentName string, singleField *models.FieldInfo, depth int) int {
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
		if subDepth := m.searchAndAppend(field.SubFieldInfo, isUpdate, parentName, singleField, depth+1); subDepth > 0 {
			return subDepth
		}
	}
	return 0 // Parent not found
}

// AddDocumentToProject adds a document to a project
func (m *MongoDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	collection := m.Database.Collection(collectionName)

	// Generate new ObjectId and set it as the Key
	objectID := primitive.NewObjectID().Hex()
	doc.Key = objectID
	doc.ID = objectID

	_, err := collection.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// UpdateDocumentOfProject updates a document in a project
func (m *MongoDriver) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {
	var collectionName string
	if param.ProjectType == models.ProjectType_SaaS {
		if param.SinglePageData {
			collectionName = fmt.Sprintf("p_%s_single_docs", param.ProjectID)
		} else {
			collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
		}
	} else {
		collectionName = fmt.Sprintf("p_%s", param.ProjectID)
	}

	collection := m.Database.Collection(collectionName)

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

	if keepRevision {
		// Generate new UUID for the document
		newUUID := uuid.New().String()
		doc.ID = newUUID
		doc.Key = newUUID

		doc.Meta.Status = param.DocPublishStatus
		// Create a new copy with new ID
		_, err := collection.InsertOne(ctx, doc)
		if err != nil {
			return err
		}

		// Get the old document
		raw, err := m.GetSingleRawDocumentFromProject(ctx, param)
		if err != nil {
			return err
		}
		oldDoc := raw.(*types.DefaultDocumentStructure)

		// Update the old document with revision info
		oldDoc.Meta.RootRevisionID = newUUID
		oldDoc.Meta.Revision = true
		oldDoc.Meta.RevisionAt = utility.GetCurrentTime()

		_, err = collection.UpdateOne(ctx, bson.M{"_id": oldDoc.ID}, bson.M{"$set": oldDoc})
		if err != nil {
			return err
		}
		return nil
	}

	if doc.Meta.Status == "" {
		doc.Meta.Status = param.DocPublishStatus
	}

	// Check if document exists for single page data
	if param.SinglePageData {
		var existingDoc types.DefaultDocumentStructure
		err := collection.FindOne(ctx, bson.M{"_id": doc.ID}).Decode(&existingDoc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// Document doesn't exist, create it
				_, err = collection.InsertOne(ctx, doc)
				if err != nil {
					return err
				}
				return nil
			}
			return err
		}
	}

	// Update or replace the document
	if replace {
		_, err := collection.ReplaceOne(ctx, bson.M{"_id": doc.ID}, doc)
		if err != nil {
			return err
		}
	} else {
		_, err := collection.UpdateOne(ctx, bson.M{"_id": doc.ID}, bson.M{"$set": doc})
		if err != nil {
			return err
		}
	}

	return nil
}

// GetProjectUser retrieves a user profile by phone, email, and project ID
func (m *MongoDriver) GetProjectUser(ctx context.Context, phone, email, projectId string) (*types.DefaultDocumentStructure, error) {
	collection := m.Database.Collection(fmt.Sprintf("p_%s", projectId))

	// Build filter for phone or email
	filter := bson.M{
		"$or": []bson.M{
			{"data.phone": phone},
			{"data.email": email},
		},
	}

	var result types.DefaultDocumentStructure
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // User not found
		}
		return nil, fmt.Errorf("failed to get project user: %w", err)
	}

	return &result, nil
}

// GetLoggedInProjectUser retrieves the logged-in user profile for the project
func (m *MongoDriver) GetLoggedInProjectUser(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	if param.DocumentID == "" {
		return nil, errors.New("user ID is required")
	}

	collection := m.Database.Collection(fmt.Sprintf("p_%s", param.ProjectID))

	filter := bson.M{"id": param.DocumentID}

	var result types.DefaultDocumentStructure
	err := collection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // User not found
		}
		return nil, fmt.Errorf("failed to get logged in project user: %w", err)
	}

	return &result, nil
}

// GetProjectUsers retrieves metadata for multiple users in the project
func (m *MongoDriver) GetProjectUsers(ctx context.Context, projectId string, keys []string) (map[string]*types.DefaultDocumentStructure, error) {
	if len(keys) == 0 {
		return make(map[string]*types.DefaultDocumentStructure), nil
	}

	collection := m.Database.Collection(fmt.Sprintf("p_%s", projectId))

	// Build filter for multiple user IDs
	filter := bson.M{
		"id": bson.M{"$in": keys},
	}

	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query project users: %w", err)
	}
	defer cursor.Close(ctx)

	result := make(map[string]*types.DefaultDocumentStructure)

	for cursor.Next(ctx) {
		var user types.DefaultDocumentStructure
		if err := cursor.Decode(&user); err != nil {
			return nil, fmt.Errorf("failed to decode user document: %w", err)
		}
		result[user.ID] = &user
	}

	if err := cursor.Err(); err != nil {
		return nil, fmt.Errorf("cursor error: %w", err)
	}

	return result, nil
}

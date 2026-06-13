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

// mongoNamespaceNotFound is MongoDB error code 26 (ns not found).
const mongoNamespaceNotFound = 26

func mongoDropCollectionIgnoringNotFound(ctx context.Context, coll *mongo.Collection) error {
	err := coll.Drop(ctx)
	if err == nil {
		return nil
	}
	var ce mongo.CommandError
	if errors.As(err, &ce) && ce.Code == mongoNamespaceNotFound {
		return nil
	}
	return err
}

// DeleteProjectBase drops the collections created by InitProjectBase for this project (p_{id}, media, relation).
func (m *MongoDriver) DeleteProjectBase(ctx context.Context, param *models.CommonSystemParams) error {
	if param == nil || strings.TrimSpace(param.ProjectID) == "" {
		return errors.New("DeleteProjectBase: project id is required")
	}
	projectId := strings.TrimSpace(param.ProjectID)
	projectCollection := fmt.Sprintf("p_%s", projectId)
	projectMediaCollection := fmt.Sprintf("%s_files", projectCollection)
	projectEdgeCollection := fmt.Sprintf("%s_relation", projectCollection)

	session, err := m.Client.StartSession()
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessCtx context.Context) (interface{}, error) {
		if err := mongoDropCollectionIgnoringNotFound(sessCtx, m.Database.Collection(projectCollection)); err != nil {
			return nil, err
		}
		if err := mongoDropCollectionIgnoringNotFound(sessCtx, m.Database.Collection(projectMediaCollection)); err != nil {
			return nil, err
		}
		if err := mongoDropCollectionIgnoringNotFound(sessCtx, m.Database.Collection(projectEdgeCollection)); err != nil {
			return nil, err
		}
		return nil, nil
	})
	return err
}

// DeleteProject removes the project’s physical collections; it delegates to DeleteProjectBase (same scope as InitProjectBase).
func (m *MongoDriver) DeleteProject(ctx context.Context, projectId string) error {
	return m.DeleteProjectBase(ctx, &models.CommonSystemParams{ProjectID: projectId})
}

// InitProjectBase creates the default media bucket, relation collection (with edge indexes), and main
// document collection p_{projectID} with standard document indexes. Driver v2 CreateCollection returns only error;
// use Database.Collection after CreateCollection.
func (m *MongoDriver) InitProjectBase(ctx context.Context, param *models.CommonSystemParams, indexes []string) error {
	if param == nil {
		return errors.New("param is required")
	}

	pid := strings.TrimSpace(param.ProjectID)
	if pid == "" {
		return errors.New("project id is required")
	}

	media := fmt.Sprintf("p_%s_files", pid)
	if err := m.Database.CreateCollection(ctx, media); err != nil {
		return err
	}

	relation := fmt.Sprintf("p_%s_relation", pid)
	if err := m.Database.CreateCollection(ctx, relation); err != nil {
		return err
	}
	relColl := m.Database.Collection(relation)
	if err := ensureMongoRelationEdgeIndexes(ctx, relColl); err != nil {
		return err
	}

	main := fmt.Sprintf("p_%s", pid)
	if err := m.Database.CreateCollection(ctx, main); err != nil {
		return err
	}
	if err := ensureMongoDocumentIndexes(ctx, m.Database.Collection(main)); err != nil {
		return err
	}

	_ = indexes // reserved for optional caller-defined indexes on the main bucket
	return nil
}

// CheckTableOrCollectionExists checks if a collection exists in the project.
// When param.Ext[models.ExtKeyRelationCollectionCheck] is true, checks relation_{model} instead of the model collection.
func (m *MongoDriver) CheckTableOrCollectionExists(ctx context.Context, param *models.CommonSystemParams) (bool, error) {
	if param == nil || param.Model == nil {
		return false, errors.New("model is required")
	}
	collectionName := param.Model.Name
	if param.Ext != nil {
		if v, ok := param.Ext[models.ExtKeyRelationCollectionCheck].(bool); ok && v {
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

// ensureMongoDocumentIndexes adds TTL, type, and publish-status indexes used on the main project document bucket.
// for each general collection we create we ahve to run this function to ensure the indexes are created
func ensureMongoDocumentIndexes(ctx context.Context, collection *mongo.Collection) error {
	ttlIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "expire_at", Value: 1}},
		Options: options.Index().SetName("doc_expire_at").SetExpireAfterSeconds(1),
	}
	if _, err := collection.Indexes().CreateOne(ctx, ttlIndex); err != nil {
		return err
	}
	typeIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "type", Value: 1}},
		Options: options.Index().SetName("doc_type"),
	}
	if _, err := collection.Indexes().CreateOne(ctx, typeIndex); err != nil {
		return err
	}
	statusIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "meta.status", Value: 1}},
		Options: options.Index().SetName("doc_publish_status"),
	}
	_, err := collection.Indexes().CreateOne(ctx, statusIndex)
	return err
}

// ensureMongoRelationEdgeIndexes adds indexes for a relation edge collection (same layout as p_{pid}_relation).
func ensureMongoRelationEdgeIndexes(ctx context.Context, collection *mongo.Collection) error {
	ttlIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "expire_at", Value: 1}},
		Options: options.Index().SetName("doc_expire_at").SetExpireAfterSeconds(1),
	}
	if _, err := collection.Indexes().CreateOne(ctx, ttlIndex); err != nil {
		return err
	}
	fromIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "from", Value: 1}},
		Options: options.Index().SetName("from_relation"),
	}
	if _, err := collection.Indexes().CreateOne(ctx, fromIndex); err != nil {
		return err
	}
	toIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "to", Value: 1}},
		Options: options.Index().SetName("to_relation"),
	}
	if _, err := collection.Indexes().CreateOne(ctx, toIndex); err != nil {
		return err
	}
	knownAsIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "known_as", Value: 1}},
		Options: options.Index().SetName("known_as_relation"),
	}
	if _, err := collection.Indexes().CreateOne(ctx, knownAsIndex); err != nil {
		return err
	}
	fromIDIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "from_id", Value: 1}},
		Options: options.Index().SetName("from_id_relation"),
	}
	if _, err := collection.Indexes().CreateOne(ctx, fromIDIndex); err != nil {
		return err
	}
	toIDIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "to_id", Value: 1}},
		Options: options.Index().SetName("to_id_relation"),
	}
	_, err := collection.Indexes().CreateOne(ctx, toIDIndex)
	return err
}

// CreateTableOrCollection creates a new table or collection in the project.
// For general project bootstrap, param.Model may be nil: in that case (and when not creating a
// relation collection) the primary bucket p_{projectID} and p_{projectID}_files are created, matching
// Arango AddCollection which does not require a model for the default collection name.
func (m *MongoDriver) CreateTableOrCollection(ctx context.Context, param *models.CommonSystemParams, indexes []string) error {
	if param == nil {
		return errors.New("param is required")
	}

	var collectionName string
	if param.Model == nil {
		pid := strings.TrimSpace(param.ProjectID)
		if pid == "" {
			return errors.New("project id is required")
		}
		main := fmt.Sprintf("p_%s", pid)
		if err := m.Database.CreateCollection(ctx, main); err != nil {
			return err
		}
		if err := ensureMongoDocumentIndexes(ctx, m.Database.Collection(main)); err != nil {
			return err
		}
		media := fmt.Sprintf("p_%s_files", pid)
		if err := m.Database.CreateCollection(ctx, media); err != nil {
			return err
		}
		return nil
	}

	isRelation := false
	var fieldIndexes []string
	for _, ix := range indexes {
		if ix == models.IndexesRelationCollectionToken {
			isRelation = true
			continue
		}
		fieldIndexes = append(fieldIndexes, ix)
	}

	collectionName = param.Model.Name
	if isRelation {
		collectionName = "relation_" + collectionName
	}

	err := m.Database.CreateCollection(ctx, collectionName)
	if err != nil {
		return err
	}

	collection := m.Database.Collection(collectionName)

	if isRelation {
		return ensureMongoRelationEdgeIndexes(ctx, collection)
	}

	for _, index := range fieldIndexes {
		indexModel := mongo.IndexModel{
			Keys:    bson.D{bson.E{Key: index, Value: 1}},
			Options: options.Index().SetName(index + "_index"),
		}
		if _, err = collection.Indexes().CreateOne(ctx, indexModel); err != nil {
			return err
		}
	}

	return ensureMongoDocumentIndexes(ctx, collection)
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
	fromCollectionName := "p_" + from + "_files"

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
	toCollectionName := "p_" + to + "_files"
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

	// Build filter
	filter := bson.M{"_id": param.DocumentID}
	mergeQueryFilterBSON(m.Conf, param, filter)

	// Get collection name
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

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
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	collection := m.Database.Collection(collectionName)
	filter := bson.M{"_id": param.DocumentID}
	mergeQueryFilterBSON(m.Conf, param, filter)

	_, err := collection.DeleteOne(ctx, filter)
	return err
}

// QueryMultiDocumentOfProject retrieves multiple documents from a project
func (m *MongoDriver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	collection := m.Database.Collection(collectionName)
	findOpts := options.Find()

	// Build filter
	filter := bson.M{}

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
	mergeQueryFilterBSON(m.Conf, param, filter)

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
	err := m.Database.CreateCollection(ctx, collectionName)
	if err != nil {
		return nil, err
	}

	collection := m.Database.Collection(collectionName)
	indexOpts := options.Index().SetName("id_index")
	indexModel := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "_id", Value: 1}},
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
		for _, existing := range targetModel.Fields {
			if existing != nil && existing.Identifier == param.FieldInfo.Identifier {
				return targetModel, nil
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
		Keys:    bson.D{bson.E{Key: fieldName, Value: 1}},
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
		Keys:    bson.D{bson.E{Key: "from", Value: 1}},
		Options: fromIndexOpts,
	}
	_, err = collection.Indexes().CreateOne(ctx, fromIndex)
	if err != nil {
		return err
	}

	// Create index for to field
	toIndexOpts := options.Index().SetName("to_index")
	toIndex := mongo.IndexModel{
		Keys:    bson.D{bson.E{Key: "to", Value: 1}},
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
			fromID := connParam.ForwardConnectionID
			toID := id

			if connParam.ConnectionType == "backward" {
				fromID = id
				toID = connParam.ForwardConnectionID
			}

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
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	collection := m.Database.Collection(collectionName)

	// Find documents that are revisions of the specified document
	filter := bson.M{
		"$or": []bson.M{
			{"meta.root_revision_id": param.DocumentID},
			{"_id": param.DocumentID},
		},
		"meta.revision": true,
	}

	// Sort by revision date descending
	findOpts := options.Find().SetSort(bson.D{bson.E{Key: "meta.revision_at", Value: -1}})

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

	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

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
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	collection := m.Database.Collection(collectionName)
	filter := bson.M{"_id": bson.M{"$in": param.DocumentIDs}}

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
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

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
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	collection := m.Database.Collection(collectionName)
	filter := bson.M{}

	if param.ResolveParams != nil {
		if where, ok := param.ResolveParams.Args["where"].(map[string]interface{}); ok {
			for k, v := range where {
				filter[k] = v
			}
		}
	}
	mergeQueryFilterBSON(m.Conf, param, filter)

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
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	collection := m.Database.Collection(collectionName)
	pipeline := mongo.Pipeline{}

	// Add match stage if filter exists
	match := bson.M{}
	if param.ResolveParams != nil {
		if where, ok := param.ResolveParams.Args["where"].(map[string]interface{}); ok {
			for k, v := range where {
				match[k] = v
			}
		}
	}
	mergeQueryFilterBSON(m.Conf, param, match)
	if len(match) > 0 {
		pipeline = append(pipeline, bson.D{bson.E{Key: "$match", Value: match}})
	}

	// Add group stage
	pipeline = append(pipeline, bson.D{
		bson.E{Key: "$group", Value: bson.D{
			bson.E{Key: "_id", Value: nil},
			bson.E{Key: "count", Value: bson.D{bson.E{Key: "$sum", Value: 1}}},
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
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	collection := m.Database.Collection(collectionName)

	filter := bson.M{}

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
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	collection := m.Database.Collection(collectionName)

	// Generate new ObjectId and set it as the Key
	objectID := primitive.NewObjectID().Hex()
	doc.Key = objectID
	doc.ID = objectID

	row := map[string]interface{}{}
	if err := runDocumentPreInsertHook(m.Conf, ctx, param, row); err != nil {
		return nil, err
	}
	mergeHookRowIntoDocData(doc, row)
	if err := runDocumentPreInsertDocHook(m.Conf, ctx, param, doc); err != nil {
		return nil, err
	}

	_, err := collection.InsertOne(ctx, doc)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

// UpdateDocumentOfProject updates a document in a project
func (m *MongoDriver) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

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
		newUUID := utility.NewID()
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
		ef := bson.M{"_id": doc.ID}
		mergeQueryFilterBSON(m.Conf, param, ef)
		err := collection.FindOne(ctx, ef).Decode(&existingDoc)
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
	upf := bson.M{"_id": doc.ID}
	mergeQueryFilterBSON(m.Conf, param, upf)
	if replace {
		_, err := collection.ReplaceOne(ctx, upf, doc)
		if err != nil {
			return err
		}
	} else {
		_, err := collection.UpdateOne(ctx, upf, bson.M{"$set": doc})
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

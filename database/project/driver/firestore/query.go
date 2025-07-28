package firestore

import (
	"context"
	"errors"
	"fmt"

	"cloud.google.com/go/firestore"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/graph-gophers/dataloader"
	strip "github.com/grokify/html-strip-tags-go"
	"github.com/tailor-inc/graphql"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type FireStoreDriver struct {
	Db *firestore.Client
}

func (a *FireStoreDriver) GetRelationDocument(ctx context.Context, cd *models.ConnectDisconnectParam) (*models.EdgeRelation, error) {
	// Get relation document from Firestore - using CurrentActionID as document ID
	collectionName := fmt.Sprintf("p_%s_relation", cd.DocCollectionName)

	doc, err := a.Db.Collection(collectionName).Doc(cd.CurrentActionID).Get(ctx)
	if err != nil {
		return nil, err
	}

	var relation models.EdgeRelation
	if err := doc.DataTo(&relation); err != nil {
		return nil, err
	}

	return &relation, nil
}

func (a *FireStoreDriver) DropModel(ctx context.Context, project *models.Project, modelName string) error {
	// Drop a model collection in Firestore
	collectionName := fmt.Sprintf("p_%s_%s", project.ID, modelName)

	// Firestore doesn't have a direct "drop collection" method, so we need to delete all documents
	batch := a.Db.Batch()
	iter := a.Db.Collection(collectionName).Documents(ctx)

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		batch.Delete(doc.Ref)
	}

	_, err := batch.Commit(ctx)
	return err
}

func (a *FireStoreDriver) CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parent_field string) error {
	// Firestore automatically creates indexes for single field queries
	// Composite indexes need to be created through Firebase Console or gcloud CLI
	// For this implementation, we'll just return nil as single field indexes are automatic
	return nil
}

func (a *FireStoreDriver) DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error {
	// Firestore indexes are managed through Firebase Console or gcloud CLI
	// Cannot be dropped programmatically through the client library
	return nil
}

func (a *FireStoreDriver) CheckCollectionExists(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) (bool, error) {
	var collectionName string
	if isRelationCollection {
		collectionName = fmt.Sprintf("p_%s_relation", param.ProjectID)
	} else {
		collectionName = fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)
	}

	// Check if collection exists by trying to get at least one document
	iter := a.Db.Collection(collectionName).Limit(1).Documents(ctx)
	_, err := iter.Next()

	if err == iterator.Done {
		// Collection exists but is empty
		return true, nil
	} else if err != nil {
		// Collection might not exist or other error
		return false, nil
	}

	// Collection exists and has documents
	return true, nil
}

func (a *FireStoreDriver) DuplicateModel(ctx context.Context, project *models.Project, modelName, newName string) (*models.ProjectSchema, error) {
	// Copy all documents from the original model to the new model
	sourceCollection := fmt.Sprintf("p_%s_%s", project.ID, modelName)
	targetCollection := fmt.Sprintf("p_%s_%s", project.ID, newName)

	// Get all documents from source collection
	iter := a.Db.Collection(sourceCollection).Documents(ctx)
	batch := a.Db.Batch()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		// Copy document to new collection
		targetRef := a.Db.Collection(targetCollection).Doc(doc.Ref.ID)
		batch.Set(targetRef, doc.Data())
	}

	// Commit the batch
	_, err := batch.Commit(ctx)
	if err != nil {
		return nil, err
	}

	// Create a new model in the schema (assuming schema is managed elsewhere)
	// For now, return the existing schema as this operation mainly copies data
	return project.Schema, nil
}

func (a *FireStoreDriver) GetProjectUsers(ctx context.Context, projectId string, keys []string) (map[string]*types.DefaultDocumentStructure, error) {
	result := make(map[string]*types.DefaultDocumentStructure)

	// Get user documents from the project collection
	collectionName := fmt.Sprintf("p_%s", projectId)

	for _, key := range keys {
		doc, err := a.Db.Collection(collectionName).Doc(key).Get(ctx)
		if err != nil {
			continue // Skip missing documents
		}

		var user types.DefaultDocumentStructure
		if err := doc.DataTo(&user); err != nil {
			continue
		}

		// Check if this is a user document
		if user.Type == "user" {
			result[key] = &user
		}
	}

	return result, nil
}

func (a *FireStoreDriver) DeleteMediaFile(ctx context.Context, param models.CommonSystemParams) error {
	// Delete media file document from Firestore
	collectionName := fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)

	_, err := a.Db.Collection(collectionName).Doc(param.DocumentID).Delete(ctx)
	return err
}

func GetFirestoreDriver(engine *models.DriverCredentials) (*FireStoreDriver, error) {

	// Sets your Google Cloud Platform project ID.
	projectID := engine.ProjectID

	ctx := context.Background()
	client, err := firestore.NewClient(ctx, projectID, option.WithCredentialsJSON([]byte(engine.FirebaseProjectCredentialJSON)))
	if err != nil {
		return nil, err
	}
	// Close client when done with
	// defer client.Close()

	return &FireStoreDriver{Db: client}, nil
}

func (a *FireStoreDriver) CheckProjectExists(ctx context.Context, projectId string) (bool, error) {
	// one firebase on project so project collection check is not necessary
	return true, nil
}

func (a *FireStoreDriver) TransferProject(ctx context.Context, userId, from, to string) error {
	return nil
}

func (a *FireStoreDriver) AddCollection(ctx context.Context, param *models.CommonSystemParams, isRelationCollection bool) error {
	projectName := param.ProjectID
	val, err := a.Db.Collection(projectName).Limit(1).Snapshots(ctx).Next()
	if err != nil {
		return err
	}
	if val.Size > 0 {
		return fmt.Errorf(`collection %s Already Exists`, projectName)
	}
	return nil
}

func (a *FireStoreDriver) AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error) {

	// if schema not found then create
	if project.Schema == nil {
		project.Schema = &models.ProjectSchema{
			Models: []*models.ModelType{model},
		}
	} else {
		var found bool
		for _, ct := range project.Schema.Models {
			if ct.Name == model.Name {
				found = true
				break
			}
		}

		if !found {
			project.Schema.Models = append(project.Schema.Models, model)
		} else {
			return nil, errors.New("model Already Defined")
		}
	}

	// check in db label also
	val, err := a.Db.Collection(model.Name).Limit(1).Snapshots(ctx).Next()
	if err != nil {
		return nil, err
	}
	if val.Size > 0 {
		return nil, errors.New(fmt.Sprintf("A model with name `%s` Already Exists in Firebase", model.Name))
	}

	return project.Schema, nil
}

func (a *FireStoreDriver) AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parent_field string) (*models.ModelType, error) {
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
	return param.Model, nil
}

func (a *FireStoreDriver) AddRelationFields(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error {
	// In Firestore, relations are typically handled by document references or subcollections
	// This is a schema-level operation that might be handled elsewhere
	// For now, we'll return nil as the relation structure is defined in the model schema
	return nil
}

func (a *FireStoreDriver) ConnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	// Connect builder functionality for Firestore
	// This might involve setting up specific collections or documents for builder integration
	collectionName := fmt.Sprintf("p_%s_builders", param.ProjectID)

	builderDoc := map[string]interface{}{
		"user_id":    param.UserID,
		"project_id": param.ProjectID,
		"connected":  true,
		"timestamp":  firestore.ServerTimestamp,
	}

	_, err := a.Db.Collection(collectionName).Doc(param.UserID).Set(ctx, builderDoc)
	return err
}

func (a *FireStoreDriver) DisconnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	// Disconnect builder functionality for Firestore
	collectionName := fmt.Sprintf("p_%s_builders", param.ProjectID)

	_, err := a.Db.Collection(collectionName).Doc(param.UserID).Delete(ctx)
	return err
}

func (a *FireStoreDriver) AddAuthAddOns(ctx context.Context, project *models.Project, auth map[string]interface{}) error {
	panic("implement me")
}

func (a *FireStoreDriver) GetProjectUser(ctx context.Context, phone, email, projectId string) (*types.DefaultDocumentStructure, error) {
	collectionName := fmt.Sprintf("p_%s", projectId)

	// Query by email if provided
	if email != "" {
		query := a.Db.Collection(collectionName).Where("email", "==", email).Where("type", "==", "user").Limit(1)
		iter := query.Documents(ctx)

		doc, err := iter.Next()
		if err == iterator.Done {
			return nil, fmt.Errorf("user not found")
		}
		if err != nil {
			return nil, err
		}

		var user types.DefaultDocumentStructure
		if err := doc.DataTo(&user); err != nil {
			return nil, err
		}
		return &user, nil
	}

	// Query by phone if provided
	if phone != "" {
		query := a.Db.Collection(collectionName).Where("phone", "==", phone).Where("type", "==", "user").Limit(1)
		iter := query.Documents(ctx)

		doc, err := iter.Next()
		if err == iterator.Done {
			return nil, fmt.Errorf("user not found")
		}
		if err != nil {
			return nil, err
		}

		var user types.DefaultDocumentStructure
		if err := doc.DataTo(&user); err != nil {
			return nil, err
		}
		return &user, nil
	}

	return nil, fmt.Errorf("email or phone must be provided")
}

func (a *FireStoreDriver) GetLoggedInProjectUser(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	collectionName := fmt.Sprintf("p_%s", param.ProjectID)

	// Get user by user ID
	doc, err := a.Db.Collection(collectionName).Doc(param.UserID).Get(ctx)
	if err != nil {
		return nil, err
	}

	var user types.DefaultDocumentStructure
	if err := doc.DataTo(&user); err != nil {
		return nil, err
	}

	// Verify this is a user document
	if user.Type != "user" {
		return nil, fmt.Errorf("document is not a user")
	}

	return &user, nil
}

func (a *FireStoreDriver) DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error {
	// Delete relation documents from the relation collection
	relationCollectionName := fmt.Sprintf("p_%s_relation", param.ProjectID)

	// Query for relations involving this document
	query := a.Db.Collection(relationCollectionName).Where("from_id", "==", param.DocumentID)
	iter := query.Documents(ctx)

	batch := a.Db.Batch()
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		batch.Delete(doc.Ref)
	}

	// Also query for relations where this document is the target
	query2 := a.Db.Collection(relationCollectionName).Where("to_id", "==", param.DocumentID)
	iter2 := query2.Documents(ctx)

	for {
		doc, err := iter2.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		batch.Delete(doc.Ref)
	}

	_, err := batch.Commit(ctx)
	return err
}

func (a *FireStoreDriver) DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	// Delete multiple documents from a project collection
	collectionName := fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)

	batch := a.Db.Batch()
	for _, docID := range param.DocumentIDs {
		docRef := a.Db.Collection(collectionName).Doc(docID)
		batch.Delete(docRef)
	}

	_, err := batch.Commit(ctx)
	return err
}

func (a *FireStoreDriver) DropField(ctx context.Context, param *models.CommonSystemParams) error {
	// Drop a field from all documents in a collection
	collectionName := fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)

	// Get all documents in the collection
	iter := a.Db.Collection(collectionName).Documents(ctx)
	batch := a.Db.Batch()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}

		// Remove the field from the document
		data := doc.Data()
		if param.FieldInfo != nil {
			delete(data, param.FieldInfo.Identifier)
			batch.Set(doc.Ref, data)
		}
	}

	_, err := batch.Commit(ctx)
	return err
}

func (a *FireStoreDriver) DeleteRelationDocuments(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error {
	panic("implement me")
}

func (a *FireStoreDriver) RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error {
	panic("implement me")
}

func (a *FireStoreDriver) ConvertModel(ctx context.Context, project *models.Project, modelName string) error {
	panic("implement me")
}

func (a *FireStoreDriver) RenameField(ctx context.Context, oldFiledName string, repeatedGroup string, param *models.CommonSystemParams) error {
	panic("implement me")
}

func (a *FireStoreDriver) GetSystemUser(ctx context.Context, id string) (*models.SystemUser, error) {
	panic("implement me")
}

func (a *FireStoreDriver) GetProject(ctx context.Context, id string) (*models.Project, error) {

	var project models.Project
	iter, err := a.Db.Collection("projects").Doc(id).Get(ctx)
	if err != nil {
		return nil, err
	}
	err = iter.DataTo(&project)
	if err != nil {
		return nil, err
	}
	return &project, nil
}

func (a *FireStoreDriver) GetProjectWithRolesAndPermission(ctx context.Context, userId string) ([]*models.ProjectWithRoles, error) {
	panic("implement me")
}

func (a *FireStoreDriver) ListProjects(ctx context.Context, userId string) ([]*models.Project, error) {
	panic("implement me")
}

func (a *FireStoreDriver) GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	panic("implement me")
}

func (a *FireStoreDriver) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	// Get a single document from the project collection
	collectionName := fmt.Sprintf("p_%s_%s", param.ProjectID, param.Model.Name)

	doc, err := a.Db.Collection(collectionName).Doc(param.DocumentID).Get(ctx)
	if err != nil {
		return nil, err
	}

	var document types.DefaultDocumentStructure
	if err := doc.DataTo(&document); err != nil {
		return nil, err
	}

	return &document, nil
}

func (a *FireStoreDriver) GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*models.DocumentRevisionHistory, error) {
	panic("implement me")
}

func (a *FireStoreDriver) GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	panic("implement me")
}

func (a *FireStoreDriver) GetAllRelationDocumentsOfSingleDocument(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error) {
	panic("implement me")
}

func (a *FireStoreDriver) SearchFunctions(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ApitoFunction], error) {
	panic("implement me")
}

func (a *FireStoreDriver) SearchWebHooks(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.Webhook], error) {
	panic("implement me")
}

func (a *FireStoreDriver) GetWebHook(ctx context.Context, projectId, hookId string) (*models.Webhook, error) {
	panic("implement me")
}

func (a *FireStoreDriver) SearchUsers(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.SystemUser], error) {
	panic("implement me")
}

func (a *FireStoreDriver) ListMedias(ctx context.Context, projectId string, param *graphql.ResolveParams) ([]*models.FileDetails, error) {
	panic("implement me")
}

func (a *FireStoreDriver) CountMedias(ctx context.Context, projectId string, param *graphql.ResolveParams) (int, error) {
	panic("implement me")
}

func (a *FireStoreDriver) CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, previewMode bool) (int, error) {
	panic("implement me")
}

func (a *FireStoreDriver) QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {

	var multilineFields []string
	for _, f := range param.Model.Fields {
		if f.FieldType == "multiline" {
			multilineFields = append(multilineFields, f.Identifier)
		}
	}
	query, err := RootResolverQueryBuilder(param, false)
	if err != nil {
		return nil, err
	}
	collection := a.Db.Collection(param.Model.Name).Query
	for _, q := range query {
		collection = q
	}

	iter := collection.Documents(ctx)

	if err != nil {
		return nil, err
	}
	var docs []*types.DefaultDocumentStructure
	for {
		rdoc, err := iter.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}

		var doc types.DefaultDocumentStructure
		rdoc.DataTo(&doc)

		for _, m := range multilineFields { // #todo if not requestd then dont run
			converter := md.NewConverter("", true, nil)
			if d, ok := doc.Data[m].(map[string]interface{}); ok {
				if html, ok := d["html"].(string); ok {
					markdown, err := converter.ConvertString(html)
					if err != nil {
						fmt.Println(err.Error())
					}
					d["markdown"] = markdown
					d["text"] = strip.StripTags(html)
				}
			}
		}
		docs = append(docs, &doc)
	}

	return []byte{}, nil
}

func (a *FireStoreDriver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {

	var multilineFields []string
	for _, f := range param.Model.Fields {
		if f.FieldType == "multiline" {
			multilineFields = append(multilineFields, f.Identifier)
		}
	}
	query, err := RootResolverQueryBuilder(param, false)
	if err != nil {
		return nil, err
	}
	collection := a.Db.Collection(param.Model.Name).Query
	for _, q := range query {
		collection = q
	}

	iter := collection.Documents(ctx)

	if err != nil {
		return nil, err
	}
	var docs []*types.DefaultDocumentStructure
	for {
		rdoc, err := iter.Next()
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}

		var doc types.DefaultDocumentStructure
		rdoc.DataTo(&doc)

		for _, m := range multilineFields { // #todo if not requestd then dont run
			converter := md.NewConverter("", true, nil)
			if d, ok := doc.Data[m].(map[string]interface{}); ok {
				if html, ok := d["html"].(string); ok {
					markdown, err := converter.ConvertString(html)
					if err != nil {
						fmt.Println(err.Error())
					}
					d["markdown"] = markdown
					d["text"] = strip.StripTags(html)
				}
			}
		}
		docs = append(docs, &doc)
	}

	return docs, nil
}

func (a *FireStoreDriver) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	panic("implement me")
}

func (a *FireStoreDriver) AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error {
	panic("implement me")
}

func (a *FireStoreDriver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberId string) error {
	panic("implement me")
}

func (a *FireStoreDriver) CreateMediaDocument(ctx context.Context, projectId string, media *models.FileDetails) (*models.FileDetails, error) {
	panic("implement me")
}

func (a *FireStoreDriver) UpdateUser(ctx context.Context, user *models.SystemUser, replace bool) error {
	panic("implement me")
}

func (a *FireStoreDriver) CheckTokenBlacklisted(ctx context.Context, tokenId string) error {
	panic("implement me")
}

func (a *FireStoreDriver) BlacklistAToken(ctx context.Context, token map[string]interface{}) error {
	panic("implement me")
}

func (a *FireStoreDriver) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {
	panic("implement me")
}

func (a *FireStoreDriver) DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	panic("implement me")
}

func (a *FireStoreDriver) DeleteProject(ctx context.Context, projectId string) error {
	panic("implement me")
}

func (a *FireStoreDriver) CreateRelation(ctx context.Context, projectId string, relation *models.EdgeRelation) error {
	panic("implement me")
}

func (a *FireStoreDriver) DeleteRelation(ctx context.Context, param *models.ConnectDisconnectParam, id string) error {
	panic("implement me")
}

func (a *FireStoreDriver) NewInsertableRelations(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	panic("implement me")
}

func (a *FireStoreDriver) CheckOneToOneRelationExists(ctx context.Context, param *models.ConnectDisconnectParam) (bool, error) {
	panic("implement me")
}

func (a *FireStoreDriver) GetRelationIds(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	panic("implement me")
}

func (a *FireStoreDriver) RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) ([]byte, error) {
	panic("implement me")
}
func (a *FireStoreDriver) RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) (interface{}, error) {
	panic("implement me")
}

func (a *FireStoreDriver) MetaDataLoader(ctx context.Context, projectId string, keys *dataloader.Keys) ([]*dataloader.Result, error) {
	panic("implement me")
}

func (a *FireStoreDriver) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	panic("implement me")
}

func (a *FireStoreDriver) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	panic("implement me")
}

func (a *FireStoreDriver) AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	panic("implement me")
}

func (a *FireStoreDriver) AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	panic("implement me")
}

func (a *FireStoreDriver) UpdateUsages(ctx context.Context, projectId string, bandwidth float64) error {
	panic("implement me")
}

package interfaces

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
)

type ProjectDBInterface interface {

	// Project-related functions
	// InitProjectCollection initializes a project collection with optional indexes
	InitProjectBase(ctx context.Context, param *models.CommonSystemParams, indexes []string) error
	// DeleteProjectBase drops the same physical storage created by InitProjectBase (base bucket/graph/tables only).
	DeleteProjectBase(ctx context.Context, param *models.CommonSystemParams) error
	// DeleteProject deletes a project by its ID.
	DeleteProject(ctx context.Context, projectID string) error
	// TransferProject transfers a project from one user to another.
	TransferProject(ctx context.Context, userId, from, to string) error

	// Collection-related functions
	// CheckCollectionExists checks if a collection exists in the project.
	CheckTableOrCollectionExists(ctx context.Context, param *models.CommonSystemParams) (bool, error)
	// AddCollection adds a new collection to the project with optional indexes
	CreateTableOrCollection(ctx context.Context, param *models.CommonSystemParams, indexes []string) error

	// Model-related functions
	// AddModel adds a new model to the project.
	AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error)
	// AddFieldToModel adds a new field to an existing model in the project.
	AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parent_field string) (*models.ModelType, error)
	// RenameModel renames a model in the project.
	RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error
	// ConvertModel converts a model in the project.
	ConvertModel(ctx context.Context, project *models.Project, modelName string) error
	// DropModel drops a model from the project.
	DropModel(ctx context.Context, project *models.Project, modelName string) error

	// Index-related functions
	// CreateIndex creates an index for a model in the project.
	CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parent_field string) error
	// DropIndex drops an index from a model in the project.
	DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error

	// Relation-related functions
	// AddRelationFields creates a relation field (has one or has many) between models.
	AddRelationFields(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error
	// DeleteRelationDocuments drops pivot tables, relation keys, or collection tables and all documents within them.
	DeleteRelationDocuments(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error
	// GetRelationDocument retrieves a relation document by ID.
	GetRelationDocument(ctx context.Context, param *models.ConnectDisconnectParam) (*models.EdgeRelation, error)
	// CreateRelation creates a relation in the project.
	CreateRelation(ctx context.Context, projectId string, relation *models.EdgeRelation) error
	// DeleteRelation deletes a relation in the project.
	DeleteRelation(ctx context.Context, param *models.ConnectDisconnectParam, id string) error
	// NewInsertableRelations retrieves new insertable relations in the project.
	NewInsertableRelations(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error)
	// CheckOneToOneRelationExists checks if a one-to-one relation exists in the project.
	CheckOneToOneRelationExists(ctx context.Context, param *models.ConnectDisconnectParam) (bool, error)
	// GetRelationIds retrieves the IDs of every document related to a document.
	GetRelationIds(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error)

	// Builder-related functions
	// ConnectBuilder connects a builder to the project.
	ConnectBuilder(ctx context.Context, param *models.CommonSystemParams) error
	// DisconnectBuilder disconnects a builder from the project.
	DisconnectBuilder(ctx context.Context, param *models.CommonSystemParams) error

	// User-related functions
	// GetProjectUser retrieves a user profile by phone, email, and project ID.
	GetProjectUser(ctx context.Context, phone, email, projectId string) (*types.DefaultDocumentStructure, error)
	// GetLoggedInProjectUser retrieves the logged-in user profile for the project.
	GetLoggedInProjectUser(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error)
	// GetProjectUsers retrieves metadata for multiple users in the project.
	GetProjectUsers(ctx context.Context, projectId string, keys []string) (map[string]*types.DefaultDocumentStructure, error)

	// Get a Relation Data of a single document by id, it could be object or array
	// GetAllRelationDocumentsOfSingleDocument retrieves all relation data of a single document by ID.
	GetAllRelationDocumentsOfSingleDocument(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error)

	// Document-related functions
	// GetSingleProjectDocumentBytes retrieves a single project document by ID as bytes.
	GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error)
	// GetSingleProjectDocument retrieves a single project document by ID.
	GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error)
	// GetSingleProjectDocumentRevisions retrieves the revision history of a single project document by ID.
	GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*models.DocumentRevisionHistory, error)
	// GetSingleRawDocumentFromProject retrieves a single raw document from the project.
	GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error)
	// QueryMultiDocumentOfProjectBytes queries multiple documents in the project and returns the result as bytes.
	QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error)
	// QueryMultiDocumentOfProject queries multiple documents in the project and returns the result as a slice of DefaultDocumentStructure.
	QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error)
	// AddDocumentToProject adds a new document to the project.
	AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error)
	// UpdateDocumentOfProject updates a particular document in the project.
	UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error
	// DeleteDocumentFromProject deletes a document from the project.
	DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error
	// DeleteDocumentsFromProject deletes multiple documents from the project.
	DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error
	// DeleteDocumentRelation deletes all relations or data in pivot tables from the project.
	DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error

	// Metadata-related functions
	// AddTeamMetaInfo adds metadata information for a team in the project.
	AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error)

	// Raw data-related functions
	// RelationshipDataLoader loads relationship data for the project.
	RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) (interface{}, error)
	// RelationshipDataLoaderBytes loads relationship data for the project and returns it as bytes.
	RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) ([]byte, error)

	// Counting documents-related functions
	// CountDocOfProject counts the documents in the project.
	CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error)
	// CountDocOfProjectBytes counts the documents in the project and returns the result as bytes.
	CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error)
	// CountMultiDocumentOfProject counts multiple documents in the project.
	CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, previewModel bool) (int, error)

	// Aggregate-related functions
	// AggregateDocOfProject aggregates the documents in the project.
	AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error)
	// AggregateDocOfProjectBytes aggregates the documents in the project and returns the result as bytes.
	AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error)

	// Field-related functions
	// DropField drops/deletes a field and its data from the project.
	DropField(ctx context.Context, param *models.CommonSystemParams) error
	// RenameField renames a field in a model along with its data key.
	RenameField(ctx context.Context, oldFieldName string, parentField string, param *models.CommonSystemParams) error
	//DuplicateModel(project *models.Project, modelName, newName string) (*models.ProjectSchema, error)

	// DuplicateField rename a field in a model along with its data key
	//DuplicateField(oldFieldName string, repeatedFieldGroup *string, param models.CommonSystemParams) error

	// Project file metadata (table: files in project DB)
	EnsureFilesTable(ctx context.Context) error
	CreateProjectFile(ctx context.Context, file *models.ProjectFile) (*models.ProjectFile, error)
	GetProjectFile(ctx context.Context, fileID string) (*models.ProjectFile, error)
	SearchProjectFiles(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ProjectFile], error)
	DeleteProjectFiles(ctx context.Context, ids []string) error
	SumProjectFilesSize(ctx context.Context) (int64, error)

	// Project auth users (reserved table: users in project DB)
	EnsureUsersTable(ctx context.Context) error
	CreateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) (*models.ProjectAuthUser, error)
	GetProjectAuthUser(ctx context.Context, userID string) (*models.ProjectAuthUser, error)
	GetProjectAuthUserByUsername(ctx context.Context, username string) (*models.ProjectAuthUser, error)
	ListProjectAuthUsersByEmail(ctx context.Context, tenantID, email string) ([]*models.ProjectAuthUser, error)
	ListProjectAuthUsersByPhone(ctx context.Context, tenantID, phone string) ([]*models.ProjectAuthUser, error)
	ListProjectAuthUsersByGoogleSub(ctx context.Context, tenantID, googleSub string) ([]*models.ProjectAuthUser, error)
	SearchProjectAuthUsers(ctx context.Context, tenantID string, limit, offset int) ([]*models.ProjectAuthUser, int, error)
	CountProjectAuthUsersByRole(ctx context.Context, tenantID string) (map[string]int, error)
	UpdateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) error
	DeleteProjectAuthUser(ctx context.Context, userID string) error
}

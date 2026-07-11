package telemetry

import (
	"context"
	"time"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
)

// WrapProjectDBWithMetrics returns inner wrapped with apito_db_* OTel recording when enabled.
func WrapProjectDBWithMetrics(cfg *models.Config, engine string, inner interfaces.ProjectDBInterface) interfaces.ProjectDBInterface {
	if inner == nil || !MetricsEnabled(cfg) {
		return inner
	}
	if engine == "" {
		engine = "unknown"
	}
	return &projectDBMetrics{inner: inner, cfg: cfg, engine: engine}
}

type projectDBMetrics struct {
	inner  interfaces.ProjectDBInterface
	cfg    *models.Config
	engine string
}

func (w *projectDBMetrics) run(ctx context.Context, op string, ddl bool, fn func() error) error {
	start := time.Now()
	err := fn()
	if ddl {
		RecordDDLApply(ctx, w.cfg, w.engine, op, err, time.Since(start))
	} else {
		RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	}
	return err
}

func (w *projectDBMetrics) run2(ctx context.Context, op string, ddl bool, fn func() (interface{}, error)) (interface{}, error) {
	start := time.Now()
	v, err := fn()
	if ddl {
		RecordDDLApply(ctx, w.cfg, w.engine, op, err, time.Since(start))
	} else {
		RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	}
	return v, err
}

func (w *projectDBMetrics) run2doc(ctx context.Context, op string, fn func() (*types.DefaultDocumentStructure, error)) (*types.DefaultDocumentStructure, error) {
	start := time.Now()
	v, err := fn()
	RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) run2docSlice(ctx context.Context, op string, fn func() ([]*types.DefaultDocumentStructure, error)) ([]*types.DefaultDocumentStructure, error) {
	start := time.Now()
	v, err := fn()
	RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) run2bytes(ctx context.Context, op string, fn func() ([]byte, error)) ([]byte, error) {
	start := time.Now()
	v, err := fn()
	RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) run2int(ctx context.Context, op string, fn func() (int, error)) (int, error) {
	start := time.Now()
	v, err := fn()
	RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) run2bool(ctx context.Context, op string, fn func() (bool, error)) (bool, error) {
	start := time.Now()
	v, err := fn()
	RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) run2strSlice(ctx context.Context, op string, fn func() ([]string, error)) ([]string, error) {
	start := time.Now()
	v, err := fn()
	RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) run2edge(ctx context.Context, op string, fn func() (*models.EdgeRelation, error)) (*models.EdgeRelation, error) {
	start := time.Now()
	v, err := fn()
	RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) run2schema(ctx context.Context, op string, fn func() (*models.ProjectSchema, error)) (*models.ProjectSchema, error) {
	start := time.Now()
	v, err := fn()
	RecordDDLApply(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) run2model(ctx context.Context, op string, fn func() (*models.ModelType, error)) (*models.ModelType, error) {
	start := time.Now()
	v, err := fn()
	RecordDDLApply(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) run2users(ctx context.Context, op string, fn func() ([]*models.SystemUser, error)) ([]*models.SystemUser, error) {
	start := time.Now()
	v, err := fn()
	RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) run2map(ctx context.Context, op string, fn func() (map[string]*types.DefaultDocumentStructure, error)) (map[string]*types.DefaultDocumentStructure, error) {
	start := time.Now()
	v, err := fn()
	RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) run2rev(ctx context.Context, op string, fn func() ([]*models.DocumentRevisionHistory, error)) ([]*models.DocumentRevisionHistory, error) {
	start := time.Now()
	v, err := fn()
	RecordDBOperation(ctx, w.cfg, w.engine, op, err, time.Since(start))
	return v, err
}

// --- interface methods ---

func (w *projectDBMetrics) InitProjectBase(ctx context.Context, param *models.CommonSystemParams, indexes []string) error {
	return w.run(ctx, "init", true, func() error { return w.inner.InitProjectBase(ctx, param, indexes) })
}

func (w *projectDBMetrics) DeleteProjectBase(ctx context.Context, param *models.CommonSystemParams) error {
	return w.run(ctx, "drop", true, func() error { return w.inner.DeleteProjectBase(ctx, param) })
}

func (w *projectDBMetrics) DeleteProject(ctx context.Context, projectID string) error {
	return w.run(ctx, "drop", true, func() error { return w.inner.DeleteProject(ctx, projectID) })
}

func (w *projectDBMetrics) TransferProject(ctx context.Context, userId, from, to string) error {
	return w.run(ctx, "transfer", false, func() error { return w.inner.TransferProject(ctx, userId, from, to) })
}

func (w *projectDBMetrics) CheckTableOrCollectionExists(ctx context.Context, param *models.CommonSystemParams) (bool, error) {
	return w.run2bool(ctx, "get", func() (bool, error) { return w.inner.CheckTableOrCollectionExists(ctx, param) })
}

func (w *projectDBMetrics) CreateTableOrCollection(ctx context.Context, param *models.CommonSystemParams, indexes []string) error {
	return w.run(ctx, "ensure", true, func() error { return w.inner.CreateTableOrCollection(ctx, param, indexes) })
}

func (w *projectDBMetrics) AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error) {
	return w.run2schema(ctx, "init", func() (*models.ProjectSchema, error) { return w.inner.AddModel(ctx, project, model) })
}

func (w *projectDBMetrics) AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parent_field string) (*models.ModelType, error) {
	return w.run2model(ctx, "ensure", func() (*models.ModelType, error) {
		return w.inner.AddFieldToModel(ctx, param, isUpdate, parent_field)
	})
}

func (w *projectDBMetrics) RenameModel(ctx context.Context, project *models.Project, modelName, newName string) error {
	return w.run(ctx, "migrate", true, func() error { return w.inner.RenameModel(ctx, project, modelName, newName) })
}

func (w *projectDBMetrics) ApplyNamingV2PhysicalMigration(ctx context.Context, projectID string, pairs []utility.NamingV2ModelRenamePair, perModelCollections bool, relationTenantModel string) error {
	inner, ok := w.inner.(utility.NamingV2PhysicalMigrator)
	if !ok {
		return nil
	}
	return w.run(ctx, "migrate", true, func() error {
		return inner.ApplyNamingV2PhysicalMigration(ctx, projectID, pairs, perModelCollections, relationTenantModel)
	})
}

func (w *projectDBMetrics) ConvertModel(ctx context.Context, project *models.Project, modelName string) error {
	return w.run(ctx, "migrate", true, func() error { return w.inner.ConvertModel(ctx, project, modelName) })
}

func (w *projectDBMetrics) DropModel(ctx context.Context, project *models.Project, modelName string) error {
	return w.run(ctx, "drop", true, func() error { return w.inner.DropModel(ctx, project, modelName) })
}

func (w *projectDBMetrics) CreateIndex(ctx context.Context, param *models.CommonSystemParams, fieldName string, parent_field string) error {
	return w.run(ctx, "ensure", true, func() error { return w.inner.CreateIndex(ctx, param, fieldName, parent_field) })
}

func (w *projectDBMetrics) DropIndex(ctx context.Context, param *models.CommonSystemParams, indexName string) error {
	return w.run(ctx, "drop", true, func() error { return w.inner.DropIndex(ctx, param, indexName) })
}

func (w *projectDBMetrics) AddRelationFields(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error {
	return w.run(ctx, "ensure", true, func() error { return w.inner.AddRelationFields(ctx, from, to) })
}

func (w *projectDBMetrics) DeleteRelationDocuments(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error {
	return w.run(ctx, "delete", false, func() error {
		return w.inner.DeleteRelationDocuments(ctx, projectId, from, to)
	})
}

func (w *projectDBMetrics) GetRelationDocument(ctx context.Context, param *models.ConnectDisconnectParam) (*models.EdgeRelation, error) {
	return w.run2edge(ctx, "get", func() (*models.EdgeRelation, error) { return w.inner.GetRelationDocument(ctx, param) })
}

func (w *projectDBMetrics) CreateRelation(ctx context.Context, projectId string, relation *models.EdgeRelation) error {
	return w.run(ctx, "insert", false, func() error { return w.inner.CreateRelation(ctx, projectId, relation) })
}

func (w *projectDBMetrics) DeleteRelation(ctx context.Context, param *models.ConnectDisconnectParam, id string) error {
	return w.run(ctx, "delete", false, func() error { return w.inner.DeleteRelation(ctx, param, id) })
}

func (w *projectDBMetrics) NewInsertableRelations(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	return w.run2strSlice(ctx, "list", func() ([]string, error) { return w.inner.NewInsertableRelations(ctx, param) })
}

func (w *projectDBMetrics) CheckOneToOneRelationExists(ctx context.Context, param *models.ConnectDisconnectParam) (bool, error) {
	return w.run2bool(ctx, "get", func() (bool, error) { return w.inner.CheckOneToOneRelationExists(ctx, param) })
}

func (w *projectDBMetrics) GetRelationIds(ctx context.Context, param *models.ConnectDisconnectParam) ([]string, error) {
	return w.run2strSlice(ctx, "list", func() ([]string, error) { return w.inner.GetRelationIds(ctx, param) })
}

func (w *projectDBMetrics) ConnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	return w.run(ctx, "ensure", true, func() error { return w.inner.ConnectBuilder(ctx, param) })
}

func (w *projectDBMetrics) DisconnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	return w.run(ctx, "drop", true, func() error { return w.inner.DisconnectBuilder(ctx, param) })
}

func (w *projectDBMetrics) GetProjectUser(ctx context.Context, phone, email, projectId string) (*types.DefaultDocumentStructure, error) {
	return w.run2doc(ctx, "get", func() (*types.DefaultDocumentStructure, error) {
		return w.inner.GetProjectUser(ctx, phone, email, projectId)
	})
}

func (w *projectDBMetrics) GetLoggedInProjectUser(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	return w.run2doc(ctx, "get", func() (*types.DefaultDocumentStructure, error) { return w.inner.GetLoggedInProjectUser(ctx, param) })
}

func (w *projectDBMetrics) GetProjectUsers(ctx context.Context, projectId string, keys []string) (map[string]*types.DefaultDocumentStructure, error) {
	return w.run2map(ctx, "list", func() (map[string]*types.DefaultDocumentStructure, error) {
		return w.inner.GetProjectUsers(ctx, projectId, keys)
	})
}

func (w *projectDBMetrics) GetAllRelationDocumentsOfSingleDocument(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error) {
	return w.run2(ctx, "get", false, func() (interface{}, error) {
		return w.inner.GetAllRelationDocumentsOfSingleDocument(ctx, from, arg)
	})
}

func (w *projectDBMetrics) GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	return w.run2bytes(ctx, "get", func() ([]byte, error) { return w.inner.GetSingleProjectDocumentBytes(ctx, param) })
}

func (w *projectDBMetrics) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {
	return w.run2doc(ctx, "get", func() (*types.DefaultDocumentStructure, error) { return w.inner.GetSingleProjectDocument(ctx, param) })
}

func (w *projectDBMetrics) GetSingleProjectDocumentRevisions(ctx context.Context, param *models.CommonSystemParams) ([]*models.DocumentRevisionHistory, error) {
	return w.run2rev(ctx, "list", func() ([]*models.DocumentRevisionHistory, error) {
		return w.inner.GetSingleProjectDocumentRevisions(ctx, param)
	})
}

func (w *projectDBMetrics) GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	return w.run2(ctx, "get", false, func() (interface{}, error) { return w.inner.GetSingleRawDocumentFromProject(ctx, param) })
}

func (w *projectDBMetrics) QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	return w.run2bytes(ctx, "list", func() ([]byte, error) { return w.inner.QueryMultiDocumentOfProjectBytes(ctx, param) })
}

func (w *projectDBMetrics) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {
	return w.run2docSlice(ctx, "list", func() ([]*types.DefaultDocumentStructure, error) {
		return w.inner.QueryMultiDocumentOfProject(ctx, param)
	})
}

func (w *projectDBMetrics) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {
	return w.run2(ctx, "insert", false, func() (interface{}, error) { return w.inner.AddDocumentToProject(ctx, param, doc) })
}

func (w *projectDBMetrics) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {
	return w.run(ctx, "update", false, func() error { return w.inner.UpdateDocumentOfProject(ctx, param, doc, replace) })
}

func (w *projectDBMetrics) DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	return w.run(ctx, "delete", false, func() error { return w.inner.DeleteDocumentFromProject(ctx, param) })
}

func (w *projectDBMetrics) DeleteDocumentsFromProject(ctx context.Context, param *models.CommonSystemParams) error {
	return w.run(ctx, "delete", false, func() error { return w.inner.DeleteDocumentsFromProject(ctx, param) })
}

func (w *projectDBMetrics) DeleteDocumentRelation(ctx context.Context, param *models.CommonSystemParams) error {
	return w.run(ctx, "delete", false, func() error { return w.inner.DeleteDocumentRelation(ctx, param) })
}

func (w *projectDBMetrics) AddTeamMetaInfo(ctx context.Context, docs []*models.SystemUser) ([]*models.SystemUser, error) {
	return w.run2users(ctx, "insert", func() ([]*models.SystemUser, error) { return w.inner.AddTeamMetaInfo(ctx, docs) })
}

func (w *projectDBMetrics) RelationshipDataLoader(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) (interface{}, error) {
	return w.run2(ctx, "get", false, func() (interface{}, error) {
		return w.inner.RelationshipDataLoader(ctx, param, connection)
	})
}

func (w *projectDBMetrics) RelationshipDataLoaderBytes(ctx context.Context, param *models.CommonSystemParams, connection map[string]interface{}) ([]byte, error) {
	return w.run2bytes(ctx, "get", func() ([]byte, error) {
		return w.inner.RelationshipDataLoaderBytes(ctx, param, connection)
	})
}

func (w *projectDBMetrics) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	return w.run2(ctx, "aggregate", false, func() (interface{}, error) { return w.inner.CountDocOfProject(ctx, param) })
}

func (w *projectDBMetrics) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	return w.run2bytes(ctx, "aggregate", func() ([]byte, error) { return w.inner.CountDocOfProjectBytes(ctx, param) })
}

func (w *projectDBMetrics) CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, previewModel bool) (int, error) {
	return w.run2int(ctx, "aggregate", func() (int, error) { return w.inner.CountMultiDocumentOfProject(ctx, param, previewModel) })
}

func (w *projectDBMetrics) AggregateDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	return w.run2(ctx, "aggregate", false, func() (interface{}, error) { return w.inner.AggregateDocOfProject(ctx, param) })
}

func (w *projectDBMetrics) AggregateDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	return w.run2bytes(ctx, "aggregate", func() ([]byte, error) { return w.inner.AggregateDocOfProjectBytes(ctx, param) })
}

func (w *projectDBMetrics) DropField(ctx context.Context, param *models.CommonSystemParams) error {
	return w.run(ctx, "drop", true, func() error { return w.inner.DropField(ctx, param) })
}

func (w *projectDBMetrics) RenameField(ctx context.Context, oldFieldName string, repeatedFieldGroup string, param *models.CommonSystemParams) error {
	return w.run(ctx, "migrate", true, func() error {
		return w.inner.RenameField(ctx, oldFieldName, repeatedFieldGroup, param)
	})
}

func (w *projectDBMetrics) EnsureFilesTable(ctx context.Context) error {
	return w.run(ctx, "migrate", true, func() error { return w.inner.EnsureFilesTable(ctx) })
}

func (w *projectDBMetrics) CreateProjectFile(ctx context.Context, file *models.ProjectFile) (*models.ProjectFile, error) {
	start := time.Now()
	v, err := w.inner.CreateProjectFile(ctx, file)
	RecordDBOperation(ctx, w.cfg, w.engine, "insert", err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) GetProjectFile(ctx context.Context, fileID string) (*models.ProjectFile, error) {
	start := time.Now()
	v, err := w.inner.GetProjectFile(ctx, fileID)
	RecordDBOperation(ctx, w.cfg, w.engine, "get", err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) SearchProjectFiles(ctx context.Context, param *models.CommonSystemParams) (*models.SearchResponse[models.ProjectFile], error) {
	start := time.Now()
	v, err := w.inner.SearchProjectFiles(ctx, param)
	RecordDBOperation(ctx, w.cfg, w.engine, "list", err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) DeleteProjectFiles(ctx context.Context, ids []string) error {
	return w.run(ctx, "delete", false, func() error { return w.inner.DeleteProjectFiles(ctx, ids) })
}

func (w *projectDBMetrics) SumProjectFilesSize(ctx context.Context) (int64, error) {
	start := time.Now()
	v, err := w.inner.SumProjectFilesSize(ctx)
	RecordDBOperation(ctx, w.cfg, w.engine, "aggregate", err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) EnsureUsersTable(ctx context.Context) error {
	return w.run(ctx, "migrate", true, func() error { return w.inner.EnsureUsersTable(ctx) })
}

func (w *projectDBMetrics) CreateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) (*models.ProjectAuthUser, error) {
	start := time.Now()
	v, err := w.inner.CreateProjectAuthUser(ctx, user)
	RecordDBOperation(ctx, w.cfg, w.engine, "insert", err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) GetProjectAuthUser(ctx context.Context, userID string) (*models.ProjectAuthUser, error) {
	start := time.Now()
	v, err := w.inner.GetProjectAuthUser(ctx, userID)
	RecordDBOperation(ctx, w.cfg, w.engine, "get", err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) GetProjectAuthUserByUsername(ctx context.Context, username string) (*models.ProjectAuthUser, error) {
	start := time.Now()
	v, err := w.inner.GetProjectAuthUserByUsername(ctx, username)
	RecordDBOperation(ctx, w.cfg, w.engine, "get", err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) ListProjectAuthUsersByEmail(ctx context.Context, tenantID, email string) ([]*models.ProjectAuthUser, error) {
	start := time.Now()
	v, err := w.inner.ListProjectAuthUsersByEmail(ctx, tenantID, email)
	RecordDBOperation(ctx, w.cfg, w.engine, "list", err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) ListProjectAuthUsersByPhone(ctx context.Context, tenantID, phone string) ([]*models.ProjectAuthUser, error) {
	start := time.Now()
	v, err := w.inner.ListProjectAuthUsersByPhone(ctx, tenantID, phone)
	RecordDBOperation(ctx, w.cfg, w.engine, "list", err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) ListProjectAuthUsersByGoogleSub(ctx context.Context, tenantID, googleSub string) ([]*models.ProjectAuthUser, error) {
	start := time.Now()
	v, err := w.inner.ListProjectAuthUsersByGoogleSub(ctx, tenantID, googleSub)
	RecordDBOperation(ctx, w.cfg, w.engine, "list", err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) SearchProjectAuthUsers(ctx context.Context, tenantID, q string, limit, offset int) ([]*models.ProjectAuthUser, int, error) {
	start := time.Now()
	v, c, err := w.inner.SearchProjectAuthUsers(ctx, tenantID, q, limit, offset)
	RecordDBOperation(ctx, w.cfg, w.engine, "list", err, time.Since(start))
	return v, c, err
}

func (w *projectDBMetrics) CountProjectAuthUsersByRole(ctx context.Context, tenantID string) (map[string]int, error) {
	start := time.Now()
	v, err := w.inner.CountProjectAuthUsersByRole(ctx, tenantID)
	RecordDBOperation(ctx, w.cfg, w.engine, "list", err, time.Since(start))
	return v, err
}

func (w *projectDBMetrics) UpdateProjectAuthUser(ctx context.Context, user *models.ProjectAuthUser) error {
	return w.run(ctx, "update", false, func() error { return w.inner.UpdateProjectAuthUser(ctx, user) })
}

func (w *projectDBMetrics) DeleteProjectAuthUser(ctx context.Context, userID string) error {
	return w.run(ctx, "delete", false, func() error { return w.inner.DeleteProjectAuthUser(ctx, userID) })
}

package interfaces

import (
	"context"
	"github.com/apito-io/engine/models"
	dataloader "github.com/graph-gophers/dataloader/v7"
	"github.com/vektah/gqlparser/v2/ast"
)

type GraphQLExecutorInterface interface {
	Init(ctx context.Context, _driver *models.InitParams) error

	GetExecutorVersion(ctx context.Context) (string, error)

	SetApplicationCache(ctx context.Context, cache *models.ApplicationCache) error
	GetApplicationCache(ctx context.Context) (*models.ApplicationCache, error)

	GetProjectDriver(ctx context.Context) (ProjectDBInterface, error)
	SetProjectDriverCredential(ctx context.Context, driverCredentials *models.DriverCredentials) error

	//GetPluginInjectableProjectDriver(ctx context.Context) (InjectedDBOperationInterface, error)

	GetSharedDBDriver(ctx context.Context) (SharedDBInterface, error)
	SetSharedDBDriver(ctx context.Context, driver SharedDBInterface) error

	GetDataLoaders(ctx context.Context) (*models.DataLoaders, error)
	DataLoaderHandler(ctx context.Context, keys []string) []*dataloader.Result[interface{}]

	SolvePublicQuery(ctx context.Context, model string, _args interface{}, selectionSet *ast.SelectionSet, cache *models.ApplicationCache) ([]byte, error)
	SolvePublicQueryCount(ctx context.Context, model string, _args interface{}, cache *models.ApplicationCache) ([]byte, error)
	SolvePublicMutation(ctx context.Context, resolverName string, _id *string, _ids []*string, status *string, local *string, userInputPayload interface{}, connect interface{}, disconnect interface{}, cache *models.ApplicationCache) ([]byte, error)

	//ConnectDisconnectParamBuilder(ctx context.Context, project *models.Project, uid string, connectionIds map[string]interface{}, modelType *models.ModelType) ([]*shared.ConnectDisconnectParam, error)
	HandlePayloadFormatting(ctx context.Context, param *models.CommonSystemParams, local string, fields []*models.FieldInfo, inputPayload map[string]interface{}, dbPayload map[string]interface{}, deltaUpdate bool) (map[string]interface{}, error)

	//UploadImageFromURL(ctx context.Context, projectId, modelName, imageUrl string) (*models.FileDetails, error)
	//HandleMediaURL(ctx context.Context, param *shared.CommonSystemParams, media map[string]interface{}) (interface{}, error)
}

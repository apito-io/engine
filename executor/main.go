package executor

import (
	"context"
	"errors"
	"fmt"

	"github.com/apito-io/engine/database"
	shardDB "github.com/apito-io/engine/database/shared"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/graph-gophers/dataloader/v7"
)

type GraphQLExecutor struct {
	param *models.InitParams
	//projectDriver     interfaces.ProjectDBInterface
	connectionManager *database.ConnectionManager
	SharedDriver      interfaces.SharedDBInterface
	Dataloaders       *models.DataLoaders
	Cache             *models.ApplicationCache
}

func GetGraphQLExecutor(cfg *models.Config, systemDB interfaces.ApitoSystemDB) *GraphQLExecutor {
	return &GraphQLExecutor{
		connectionManager: database.NewConnectionManager(cfg, 1000, systemDB),
		Dataloaders:       &models.DataLoaders{},
	}
}

func (s *GraphQLExecutor) Init(ctx context.Context, _driver *models.InitParams) error {
	/*if s.ProjectDriver == nil && _driver.ProjectDB != nil {
		projectDriver, err := project.GetProjectDriver(_driver.ProjectDB)
		if err != nil {
			fmt.Println("project db error:", err.Error())
			return err
		}
		s.ProjectDriver = projectDriver
	}*/
	if _driver.ProjectDB != nil {
		// store project driver credentials
		s.connectionManager.AddDriverCredentials(ctx, _driver.ProjectDB)
	}

	if s.SharedDriver == nil && _driver.SharedDB != nil {
		driver, err := shardDB.GetSharedDriver(_driver.SharedDB)
		if err != nil {
			fmt.Println("shared db error:", err.Error())
			//return err
		}
		s.SharedDriver = driver
	}

	if s.Dataloaders == nil {
		s.Dataloaders = &models.DataLoaders{
			MultiLoader: dataloader.NewBatchedLoader(s.DataLoaderHandler, dataloader.WithClearCacheOnBatch[string, interface{}]()),
		}
	}
	return nil
}

/*func (s *GraphQLExecutor) GetProjectDriver(ctx context.Context) interfaces.ProjectDBInterface {
	if s.ProjectDriver == nil {
		err := s.Init(ctx, s.param)
		if err != nil {
			return nil
		}
	}
	return s.ProjectDriver
}*/

func (s *GraphQLExecutor) SetProjectDriverCredential(ctx context.Context, driverCredentials *models.DriverCredentials) error {
	s.connectionManager.AddDriverCredentials(ctx, driverCredentials)
	// now that i am using connection manager, i don't need to set the driver
	return nil
}

func (s *GraphQLExecutor) GetProjectDriver(ctx context.Context) (interfaces.ProjectDBInterface, error) {

	projectID := ctx.Value("project_id")

	if projectID == nil {
		return nil, errors.New("project id is required in context for `GetProjectDriver`")
	}
	conn, err := s.connectionManager.GetConnection(ctx, projectID.(string))
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	return conn.DBConn, nil
}

/* func (s *GraphQLExecutor) GetPluginInjectableProjectDriver(ctx context.Context) (interfaces.InjectedDBOperationInterface, error) {

	projectID := ctx.Value("project_id")

	if projectID == nil {
		return nil, errors.New("project id not found in context")
	}

	conn, err := s.connectionManager.GetConnection(ctx, projectID.(string))
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}
	return conn.PluginInjectableFunctions, nil
} */

func (s *GraphQLExecutor) GetExecutorVersion(ctx context.Context) (string, error) {
	return "v2.0", nil
}

func (s *GraphQLExecutor) GetApplicationCache(ctx context.Context) (*models.ApplicationCache, error) {
	return s.Cache, nil
}

func (s *GraphQLExecutor) SetApplicationCache(ctx context.Context, cache *models.ApplicationCache) error {
	s.Cache = cache
	return nil
}

func (s *GraphQLExecutor) GetSharedDBDriver(ctx context.Context) (interfaces.SharedDBInterface, error) {
	if s.SharedDriver == nil {
		err := s.Init(ctx, s.param)
		if err != nil {
			return nil, err
		}
	}
	return s.SharedDriver, nil
}

func (s *GraphQLExecutor) SetSharedDBDriver(ctx context.Context, driver interfaces.SharedDBInterface) error {
	s.SharedDriver = driver
	return nil
}

func (s *GraphQLExecutor) GetDataLoaders(ctx context.Context) (*models.DataLoaders, error) {
	return s.Dataloaders, nil
}

func (s *GraphQLExecutor) HandleMediaURL(ctx context.Context, media map[string]interface{}) (interface{}, error) {
	var fileDetails models.FileDetails
	if imageUrl, ok := media["url"].(string); ok && imageUrl != "" { // if it's a string then it's from user api
		fileDetails.URL = imageUrl
	}
	if imageId, ok := media["id"].(string); ok && imageId != "" { // if it's a string then it's from user api
		fileDetails.ID = imageId
	}
	if fileName, ok := media["file_name"].(string); ok && fileName != "" { // if it's a string then it's from user api
		fileDetails.FileName = fileName
	}
	return fileDetails, nil
}

var GQLServerExecutor GraphQLExecutor

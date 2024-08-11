package executor

import (
	"context"
	"fmt"

	"github.com/apito-io/buffers/interfaces"
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	"github.com/apito-io/databasedriver/project"
	"github.com/graph-gophers/dataloader/v7"
)

type GraphQLExecutor struct {
	param         *protobuff.InitParams
	ProjectDriver interfaces.ProjectDBInterface
	Dataloaders   *shared.DataLoaders
	Cache         *shared.ApplicationCache
}

func GetGraphQLExecutor() *GraphQLExecutor {
	return &GraphQLExecutor{
		Dataloaders: &shared.DataLoaders{},
	}
}

func (s *GraphQLExecutor) Init(ctx context.Context, _driver *protobuff.InitParams) error {
	if s.ProjectDriver == nil && _driver.ProjectDB != nil {
		projectDriver, err := project.GetProjectDriver(_driver.ProjectDB)
		if err != nil {
			fmt.Println("project db error:", err.Error())
			return err
		}
		s.ProjectDriver = projectDriver
		err = projectDriver.RunMigration(ctx, _driver.ProjectID)
		if err != nil {
			return err
		}
	}

	if s.Dataloaders == nil {
		s.Dataloaders = &shared.DataLoaders{
			MultiLoader: dataloader.NewBatchedLoader(s.DataLoaderHandlr, dataloader.WithClearCacheOnBatch[string, interface{}]()),
		}
	}
	return nil
}

func (s *GraphQLExecutor) GetExecutorVersion(ctx context.Context) (string, error) {
	return "v2.0", nil
}

func (s *GraphQLExecutor) GetApplicationCache(ctx context.Context) *shared.ApplicationCache {
	return s.Cache
}

func (s *GraphQLExecutor) SetApplicationCache(ctx context.Context, cache *shared.ApplicationCache) {
	s.Cache = cache
}

func (s *GraphQLExecutor) GetProjectDriver(ctx context.Context) interfaces.ProjectDBInterface {
	if s.ProjectDriver == nil {
		err := s.Init(ctx, s.param)
		if err != nil {
			return nil
		}
	}
	return s.ProjectDriver
}

func (s *GraphQLExecutor) SetProjectDriver(ctx context.Context, driver interfaces.ProjectDBInterface) {
	s.ProjectDriver = driver
}

func (s *GraphQLExecutor) GetDataloaders(ctx context.Context) *shared.DataLoaders {
	return s.Dataloaders
}

func (s *GraphQLExecutor) HandleMediaURL(ctx context.Context, media map[string]interface{}) (interface{}, error) {
	if imageUrl, ok := media["url"].(string); ok && imageUrl != "" { // if it's a string then it's from user api
		// upload the picture from the url
		return &protobuff.FileDetails{
			Id:       media["id"].(string),
			FileName: media["file_name"].(string),
			Url:      media["url"].(string),
		}, nil
	}
	return media, nil
}

var GQLServerExecutor GraphQLExecutor

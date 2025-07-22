package interfaces

import (
	"context"

	"github.com/apito-io/engine/models"
)

type CacheDBInterface interface {
	GetProject(ctx context.Context, projectID string) (*models.Project, error)
	SaveProject(ctx context.Context, project *models.Project) (*models.Project, error)

	ListKeys(ctx context.Context) ([]string, error)

	PutAppCache(ctx context.Context, projectID string, cache *models.ApplicationCache) error
	GetAppCache(ctx context.Context, projectID string) (*models.ApplicationCache, error)
	Expire(ctx context.Context, id string) error

	Put(ctx context.Context, id string, cache interface{}) error
	Get(ctx context.Context, id string) (interface{}, error)
}

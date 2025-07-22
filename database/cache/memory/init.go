package memoryCache

import (
	"context"
	"errors"
	"fmt"

	"github.com/apito-io/engine/models"
	"github.com/puzpuzpuz/xsync/v3"
)

type CacheDriver struct {
	Cache *xsync.Map
}

func GetMemoryCacheDriver(cfg *models.Config) (*CacheDriver, error) {
	return &CacheDriver{Cache: xsync.NewMap()}, nil
}

func (b *CacheDriver) Put(ctx context.Context, id string, cache interface{}) error {
	b.Cache.Store(id, cache)
	return nil
}

func (b *CacheDriver) Get(ctx context.Context, id string) (interface{}, error) {
	if val, ok := b.Cache.Load(id); ok && val != nil {
		return val, nil
	}
	return nil, errors.New("cache not found. fetch one")
}

func (b *CacheDriver) ListKeys(ctx context.Context) ([]string, error) {
	var _keys []string
	b.Cache.Range(func(key string, value interface{}) bool {
		_keys = append(_keys, key)
		return true
	})
	return _keys, nil
}

func (b *CacheDriver) GetAppCache(ctx context.Context, projectId string) (*models.ApplicationCache, error) {
	return nil, nil
}

func (b *CacheDriver) PutAppCache(ctx context.Context, projectId string, cache *models.ApplicationCache) error {
	return nil
}

func (b *CacheDriver) Expire(ctx context.Context, id string) error {
	b.Cache.Delete(id)
	return nil
}

func (b *CacheDriver) GetProject(ctx context.Context, projectId string) (*models.Project, error) {
	var _project *models.Project
	if val, ok := b.Cache.Load(projectId); ok && val != nil {
		_project = val.(*models.Project)
		return _project, nil
	}
	return nil, errors.New("project cache not found. fetch one")
}

func (b *CacheDriver) SaveProject(ctx context.Context, project *models.Project) (*models.Project, error) {
	b.Cache.Store(project.ID, project)
	return project, nil
}

func (b *CacheDriver) idMaker(projectId string) string {
	return fmt.Sprintf(`%s`, projectId)
}

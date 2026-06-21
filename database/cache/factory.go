package cache

import (
	"github.com/apito-io/engine/database"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// CreateCacheDriver creates a cache driver via the injected CacheFactory.
func CreateCacheDriver(cfg *models.Config, engineType string) (interfaces.CacheDBInterface, error) {
	f, err := database.ResolveCacheFactory(cfg)
	if err != nil {
		return nil, err
	}
	return f.Create(cfg, engineType)
}

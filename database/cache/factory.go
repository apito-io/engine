package cache

import (
	"github.com/apito-io/engine/database/cache/badger"
	"github.com/apito-io/engine/database/cache/bbolt"
	"github.com/apito-io/engine/database/cache/memory"
	"github.com/apito-io/engine/database/cache/redis"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// CreateCacheDriver creates a cache driver instance based on the engine type
func CreateCacheDriver(engineType string, cfg *models.Config) (interfaces.CacheDBInterface, error) {
	switch engineType {
	case "redis":
		return redis.GetRedisCacheDriver(cfg)
	case "badger":
		return badger.GetBadgerCacheDriver(cfg)
	case "coreDB", "bbolt", "bolt":
		return bbolt.GetBoltCacheDriver(cfg)
	case "memory":
		return memory.GetMemoryCacheDriver(cfg)
	default:
		return memory.GetMemoryCacheDriver(cfg) // Default to memory
	}
}

package cache

import (
	"log"
	"github.com/apito-io/engine/database/cache/bbolt"
	"github.com/apito-io/engine/database/cache/memory"
	"github.com/apito-io/engine/database/cache/redis"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// CreateCacheDriver creates a cache driver instance based on the engine type
func CreateCacheDriver(cfg *models.Config, engineType string) (interfaces.CacheDBInterface, error) {
	log.Printf("Creating cache driver: %s", engineType)
	switch engineType {
	case "redis":
		return redis.GetRedisCacheDriver(cfg)
	case "coredb", "coreDB", "bbolt", "bolt":
		return bbolt.GetBoltCacheDriver(cfg)
	case "memory":
		return memory.GetMemoryCacheDriver(cfg)
	default:
		return memory.GetMemoryCacheDriver(cfg) // Default to memory
	}
}

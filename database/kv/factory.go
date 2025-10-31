package kv

import (
	"fmt"
	"log"

	"github.com/apito-io/engine/database/kv/bbolt"
	"github.com/apito-io/engine/database/kv/memory"
	"github.com/apito-io/engine/database/kv/redis"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// CreateKVDriver creates a queue engine instance based on the engine type
func CreateKVDriver(engineType string, cfg *models.Config) (interfaces.KeyValueServiceInterface, error) {
	log.Printf("Creating KV driver: %s", engineType)
	switch engineType {
	case "redis":
		return redis.GetKVRedisDriver(cfg)
	case "coreDB", "bbolt", "bolt":
		return bbolt.GetKVBoltDriver(cfg)
	case "memory":
		return memory.GetKVMemoryDriver(cfg)
	default:
		return nil, fmt.Errorf("unsupported queue engine: %s", engineType)
	}
}

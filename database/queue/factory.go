package queue

import (
	"fmt"
	"log"

	"github.com/apito-io/engine/database/queue/bbolt"
	"github.com/apito-io/engine/database/queue/memory"
	"github.com/apito-io/engine/database/queue/redis"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// CreateQueueEngine creates a queue engine instance based on the engine type
func CreateQueueEngine(engineType string, cfg *models.Config) (interfaces.QueueEngineInterface, error) {
	log.Printf("Creating queue engine: %s", engineType)
	switch engineType {
	case "redis":
		return redis.GetRedisQueueDriver(cfg)
	case "coreDB", "bbolt", "bolt":
		return bbolt.GetBoltQueueDriver(cfg)
	case "memory":
		return memory.GetMemoryQueueDriver(cfg)
	default:
		return nil, fmt.Errorf("unsupported queue engine: %s", engineType)
	}
}

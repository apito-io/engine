package interfaces

import (
	"github.com/apito-io/engine/models"
)

// RealtimeBusFactory creates a RealtimeBus implementation for the configured engine.
type RealtimeBusFactory interface {
	Create(engineType string, cfg *models.Config) (RealtimeBus, error)
}

// CacheFactory creates a CacheDBInterface for the configured engine.
type CacheFactory interface {
	Create(cfg *models.Config, engineType string) (CacheDBInterface, error)
}

// KVFactory creates a KeyValueServiceInterface for the configured engine.
type KVFactory interface {
	Create(engineType string, cfg *models.Config) (KeyValueServiceInterface, error)
}

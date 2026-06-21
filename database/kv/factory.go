package kv

import (
	"github.com/apito-io/engine/database"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// CreateKVDriver creates a KV driver via the injected KVFactory.
func CreateKVDriver(engineType string, cfg *models.Config) (interfaces.KeyValueServiceInterface, error) {
	f, err := database.ResolveKVFactory(cfg)
	if err != nil {
		return nil, err
	}
	return f.Create(engineType, cfg)
}

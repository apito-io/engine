package realtime

import (
	"github.com/apito-io/engine/database"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// CreateRealtimeBus builds a realtime fan-out bus via the injected RealtimeBusFactory.
func CreateRealtimeBus(engineType string, cfg *models.Config) (interfaces.RealtimeBus, error) {
	f, err := database.ResolveRealtimeBusFactory(cfg)
	if err != nil {
		return nil, err
	}
	return f.Create(engineType, cfg)
}

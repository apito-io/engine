package realtime

import (
	"fmt"
	"log"

	"github.com/apito-io/engine/database/realtime/memory"
	"github.com/apito-io/engine/database/realtime/nats"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// CreateRealtimeBus builds a realtime fan-out bus based on the engine type.
// Unlike the durable queue, realtime delivery is best-effort and optimized for
// low-latency fan-out to many websocket subscribers.
func CreateRealtimeBus(engineType string, cfg *models.Config) (interfaces.RealtimeBus, error) {
	log.Printf("Creating realtime bus: %s", engineType)
	switch engineType {
	case "nats":
		return nats.GetNatsRealtimeBus(cfg)
	case "memory", "":
		return memory.GetMemoryRealtimeBus(cfg)
	default:
		return nil, fmt.Errorf("unsupported realtime engine: %s", engineType)
	}
}

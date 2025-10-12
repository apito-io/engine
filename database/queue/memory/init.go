package memory

import (
	"sync"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/gochannel"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// MemoryQueueService implements QueueEngineInterface using Watermill GoChannel Pub/Sub
type MemoryQueueService struct {
	publisher   message.Publisher
	subscriber  message.Subscriber
	logger      watermill.LoggerAdapter
	pubSub      *gochannel.GoChannel
	subscribers map[string]*models.Subscriber
	mutex       sync.RWMutex
}

// Ensure MemoryQueueService implements QueueEngineInterface
var _ interfaces.QueueEngineInterface = (*MemoryQueueService)(nil)

func GetMemoryQueueDriver(cfg *models.Config) (*MemoryQueueService, error) {
	logger := watermill.NewStdLogger(false, false)

	// Create GoChannel producer/consumer
	pubSub := gochannel.NewGoChannel(gochannel.Config{
		OutputChannelBuffer:            100,   // Buffer size for output channels
		Persistent:                     true,  // Keep messages until acknowledged
		BlockPublishUntilSubscriberAck: false, // Don't block publishing
		PreserveContext:                false, // Don't preserve context for performance
	}, logger)

	return &MemoryQueueService{
		publisher:   pubSub,
		subscriber:  pubSub,
		logger:      logger,
		pubSub:      pubSub,
		subscribers: make(map[string]*models.Subscriber),
	}, nil
}

package redis

import (
	"fmt"
	"strconv"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-redisstream/pkg/redisstream"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/redis/go-redis/v9"

	// Ensure we implement the interface
	_ "github.com/apito-io/engine/interfaces"
)

// RedisQueueService implements QueueEngineInterface using Watermill RedisStream
type RedisQueueService struct {
	publisher  message.Publisher
	subscriber message.Subscriber
	logger     watermill.LoggerAdapter
	client     *redis.Client
}

// Ensure RedisQueueService implements QueueEngineInterface
var _ interfaces.QueueEngineInterface = (*RedisQueueService)(nil)

func GetRedisQueueDriver(cfg *models.Config) (*RedisQueueService, error) {
	dbNo, err := strconv.Atoi(cfg.QueueStorageEngineDatabase)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.QueueStorageEngineHost, cfg.QueueStorageEnginePort),
		Password: cfg.QueueStorageEnginePassword,
		DB:       dbNo,
	})

	logger := watermill.NewStdLogger(false, false)

	// Create Redis Stream Publisher
	publisher, err := redisstream.NewPublisher(
		redisstream.PublisherConfig{
			Client:     client,
			Marshaller: redisstream.DefaultMarshallerUnmarshaller{},
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis publisher: %w", err)
	}

	// Create Redis Stream Subscriber
	subscriber, err := redisstream.NewSubscriber(
		redisstream.SubscriberConfig{
			Client:        client,
			Unmarshaller:  redisstream.DefaultMarshallerUnmarshaller{},
			ConsumerGroup: "apito-consumer-group",
		},
		logger,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create redis subscriber: %w", err)
	}

	return &RedisQueueService{
		publisher:  publisher,
		subscriber: subscriber,
		logger:     logger,
		client:     client,
	}, nil
}

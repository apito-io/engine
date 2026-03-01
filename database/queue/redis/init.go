package redis

import (
	"context"
	"fmt"
	"strconv"
	"sync"

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
	publisher     message.Publisher
	subscriber    message.Subscriber
	logger        watermill.LoggerAdapter
	client        *redis.Client
	consumerGroup string
	topics        sync.Map // tracks subscribed topics for NOGROUP recovery
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

	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

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
		client.Close()
		return nil, fmt.Errorf("failed to create redis publisher: %w", err)
	}

	consumerGroup := "apito-consumer-group"

	svc := &RedisQueueService{
		publisher:     publisher,
		subscriber:    nil, // set below
		logger:        logger,
		client:        client,
		consumerGroup: consumerGroup,
	}

	// Create Redis Stream Subscriber with NOGROUP recovery callback
	subscriber, err := redisstream.NewSubscriber(
		redisstream.SubscriberConfig{
			Client:                 client,
			Unmarshaller:           redisstream.DefaultMarshallerUnmarshaller{},
			ConsumerGroup:          consumerGroup,
			ShouldStopOnReadErrors: svc.handleReadError,
		},
		logger,
	)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create redis subscriber: %w", err)
	}

	svc.subscriber = subscriber
	return svc, nil
}

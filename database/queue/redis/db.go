package redis

import (
	"context"
	"fmt"
	"strings"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/apito-io/engine/models"
)

// Publish implements message.Publisher interface
func (r *RedisQueueService) Publish(topic string, messages ...*message.Message) error {
	return r.publisher.Publish(topic, messages...)
}

// handleReadError is the ShouldStopOnReadErrors callback for the Watermill subscriber.
// When NOGROUP occurs (e.g. after Redis restart/flush), recreates consumer groups for all tracked topics.
// Returns false to continue retrying instead of stopping the subscriber.
func (r *RedisQueueService) handleReadError(err error) bool {
	if !strings.Contains(err.Error(), "NOGROUP") {
		return false
	}
	r.topics.Range(func(key, value interface{}) bool {
		topic := key.(string)
		createErr := r.client.XGroupCreateMkStream(context.Background(), topic, r.consumerGroup, "0").Err()
		if createErr != nil && createErr.Error() != "BUSYGROUP Consumer Group name already exists" {
			r.logger.Error("Failed to recreate consumer group", createErr, nil)
		}
		return true
	})
	return false
}

// ensureConsumerGroup creates the consumer group for a topic if it doesn't exist
// This is required for Redis Streams consumer groups to work properly
func (r *RedisQueueService) ensureConsumerGroup(ctx context.Context, topic string) error {
	// Try to create the consumer group with MKSTREAM option using raw command
	// This will create both the stream and the group if they don't exist
	// Command: XGROUP CREATE stream group start [MKSTREAM]
	err := r.client.Do(ctx, "XGROUP", "CREATE", topic, r.consumerGroup, "0", "MKSTREAM").Err()
	if err != nil {
		// If the group already exists, that's fine - we can ignore BUSYGROUP error
		// Redis returns "BUSYGROUP Consumer Group name already exists" if the group exists
		errStr := err.Error()
		if errStr != "BUSYGROUP Consumer Group name already exists" {
			return fmt.Errorf("failed to create consumer group for topic %s: %w", topic, err)
		}
		// Group already exists, which is fine
	}
	return nil
}

// Subscribe implements message.Subscriber interface
func (r *RedisQueueService) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	r.topics.Store(topic, true)

	// Ensure the consumer group exists before subscribing
	// This prevents the NOGROUP error
	if err := r.ensureConsumerGroup(ctx, topic); err != nil {
		return nil, fmt.Errorf("failed to ensure consumer group for topic %s: %w", topic, err)
	}

	return r.subscriber.Subscribe(ctx, topic)
}

// Close implements QueueEngineInterface.Close
func (r *RedisQueueService) Close() error {
	var err error

	if closeErr := r.publisher.Close(); closeErr != nil {
		err = closeErr
	}

	if closeErr := r.subscriber.Close(); closeErr != nil {
		if err == nil {
			err = closeErr
		}
	}

	if closeErr := r.client.Close(); closeErr != nil {
		if err == nil {
			err = closeErr
		}
	}

	return err
}

// AddSubscriber method adds a subscriber to the Pub/Sub service and returns a pointer to the Subscriber object and an error if the operation fails.
func (s *RedisQueueService) AddSubscriber(ctx context.Context, userID string) (*models.Subscriber, error) {
	subscriber := &models.Subscriber{
		UserID:   userID,
		IsActive: true,
	}

	// Store subscriber info in Redis (using a namespace to avoid conflicts)
	key := "apito:subscribers:" + userID
	return subscriber, s.client.Set(ctx, key, "active", 0).Err()
}

// RemoveSubscriber method removes a subscriber from the Pub/Sub service and returns an error if the operation fails.
func (s *RedisQueueService) RemoveSubscriber(ctx context.Context, userID string) error {
	key := "apito:subscribers:" + userID
	return s.client.Del(ctx, key).Err()
}

// GetSubscriber method retrieves a subscriber from the Pub/Sub service and returns a pointer to the Subscriber object and an error if the operation fails.
func (s *RedisQueueService) GetSubscriber(ctx context.Context, userID string) (*models.Subscriber, error) {
	key := "apito:subscribers:" + userID
	val, err := s.client.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	subscriber := &models.Subscriber{
		UserID:   userID,
		IsActive: val == "active",
	}
	return subscriber, nil
}

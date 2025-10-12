package bbolt

import (
	"context"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/apito-io/engine/models"
)

// Publish implements message.Publisher interface
func (r *BoltQueueService) Publish(topic string, messages ...*message.Message) error {
	return r.publisher.Publish(topic, messages...)
}

// Subscribe implements message.Subscriber interface
func (r *BoltQueueService) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	return r.subscriber.Subscribe(ctx, topic)
}

// Close implements QueueEngineInterface.Close
func (r *BoltQueueService) Close() error {
	var err error

	if closeErr := r.publisher.Close(); closeErr != nil {
		err = closeErr
	}

	if closeErr := r.subscriber.Close(); closeErr != nil {
		if err == nil {
			err = closeErr
		}
	}

	return err
}

// AddSubscriber method adds a subscriber to the Pub/Sub service and returns a pointer to the Subscriber object and an error if the operation fails.
func (s *BoltQueueService) AddSubscriber(ctx context.Context, userID string) (*models.Subscriber, error) {
	subscriber := &models.Subscriber{
		UserID:   userID,
		IsActive: true,
	}

	// For BoltDB, we'll need to implement a simple key-value store for subscribers
	// This is a basic implementation using topic-based storage
	msg := message.NewMessage(userID, []byte("active"))
	return subscriber, s.Publish("apito:subscribers", msg)
}

// RemoveSubscriber method removes a subscriber from the Pub/Sub service and returns an error if the operation fails.
func (s *BoltQueueService) RemoveSubscriber(ctx context.Context, userID string) error {
	// For BoltDB, publish an "inactive" message
	msg := message.NewMessage(userID, []byte("inactive"))
	return s.Publish("apito:subscribers", msg)
}

// GetSubscriber method retrieves a subscriber from the Pub/Sub service and returns a pointer to the Subscriber object and an error if the operation fails.
func (s *BoltQueueService) GetSubscriber(ctx context.Context, userID string) (*models.Subscriber, error) {
	// For BoltDB implementation, this is simplified - in a real implementation,
	// you'd need to maintain a separate subscriber registry
	subscriber := &models.Subscriber{
		UserID:   userID,
		IsActive: true, // Assume active for BoltDB simplicity
	}
	return subscriber, nil
}

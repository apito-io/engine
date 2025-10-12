package memory

import (
	"context"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/apito-io/engine/models"
)

// Publish implements message.Publisher interface
func (m *MemoryQueueService) Publish(topic string, messages ...*message.Message) error {
	return m.publisher.Publish(topic, messages...)
}

// Subscribe implements message.Subscriber interface
func (m *MemoryQueueService) Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error) {
	return m.subscriber.Subscribe(ctx, topic)
}

// Close implements QueueEngineInterface.Close
func (m *MemoryQueueService) Close() error {
	var err error

	if closeErr := m.publisher.Close(); closeErr != nil {
		err = closeErr
	}

	if closeErr := m.subscriber.Close(); closeErr != nil {
		if err == nil {
			err = closeErr
		}
	}

	return err
}

// AddSubscriber method adds a subscriber to the Pub/Sub service and returns a pointer to the Subscriber object and an error if the operation fails.
func (m *MemoryQueueService) AddSubscriber(ctx context.Context, userID string) (*models.Subscriber, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	subscriber := &models.Subscriber{
		UserID:   userID,
		IsActive: true,
		Data:     make(chan interface{}, 100), // Buffered channel for data
	}

	m.subscribers[userID] = subscriber

	// Publish notification that subscriber was added
	msg := message.NewMessage(userID, []byte("active"))
	m.Publish("apito:memory-notifications", msg)

	return subscriber, nil
}

// RemoveSubscriber method removes a subscriber from the Pub/Sub service and returns an error if the operation fails.
func (m *MemoryQueueService) RemoveSubscriber(ctx context.Context, userID string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if subscriber, exists := m.subscribers[userID]; exists {
		close(subscriber.Data)
		delete(m.subscribers, userID)

		// Publish notification that subscriber was removed
		msg := message.NewMessage(userID, []byte("inactive"))
		m.Publish("apito:memory-notifications", msg)
	}

	return nil
}

// GetSubscriber method retrieves a subscriber from the Pub/Sub service and returns a pointer to the Subscriber object and an error if the operation fails.
func (m *MemoryQueueService) GetSubscriber(ctx context.Context, userID string) (*models.Subscriber, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if subscriber, exists := m.subscribers[userID]; exists {
		return subscriber, nil
	}

	return nil, models.ErrSubscriberNotFound
}

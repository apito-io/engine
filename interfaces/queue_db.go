package interfaces

import (
	"context"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/apito-io/engine/models"
)

// QueueEngineInterface is an interface that defines the methods for interacting with a Pub/Sub service using Watermill.
type QueueEngineInterface interface {
	message.Publisher
	message.Subscriber

	// Close closes the queue engine connection
	Close() error

	// AddSubscriber method adds a subscriber to the Pub/Sub service and returns a pointer to the Subscriber object and an error if the operation fails.
	AddSubscriber(ctx context.Context, userID string) (*models.Subscriber, error)

	// RemoveSubscriber method removes a subscriber from the Pub/Sub service and returns an error if the operation fails.
	RemoveSubscriber(ctx context.Context, userID string) error

	// GetSubscriber method retrieves a subscriber from the Pub/Sub service and returns a pointer to the Subscriber object and an error if the operation fails.
	GetSubscriber(ctx context.Context, userID string) (*models.Subscriber, error)
}
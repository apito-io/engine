package interfaces

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/redis/go-redis/v9"
)

// PubSubServiceInterface is an interface that defines the methods for interacting with a Pub/Sub service.
type PubSubServiceInterface interface {
	// Subscribe method subscribes to a Redis channel and returns a pointer to a PubSub object.
	Subscribe(ctx context.Context, chanel string) *redis.PubSub

	// Publish method publishes data to a Redis channel and returns an error if the operation fails.
	Publish(ctx context.Context, chanel string, data interface{}) error

	// AddSubscriber method adds a subscriber to the Pub/Sub service and returns a pointer to the Subscriber object and an error if the operation fails.
	AddSubscriber(ctx context.Context, userID string) (*models.Subscriber, error)

	// RemoveSubscriber method removes a subscriber from the Pub/Sub service and returns an error if the operation fails.
	RemoveSubscriber(ctx context.Context, userID string) error

	// GetSubscriber method retrieves a subscriber from the Pub/Sub service and returns a pointer to the Subscriber object and an error if the operation fails.
	GetSubscriber(ctx context.Context, userID string) (*models.Subscriber, error)
}

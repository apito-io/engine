package redis

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/redis/go-redis/v9"
)

func (s *RedisQueueService) AddSubscriber(ctx context.Context, userID string) (*models.Subscriber, error) {
	//TODO implement me
	panic("implement me")
}

func (s *RedisQueueService) RemoveSubscriber(ctx context.Context, userID string) error {
	//TODO implement me
	panic("implement me")
}

func (s *RedisQueueService) GetSubscriber(ctx context.Context, userID string) (*models.Subscriber, error) {
	//TODO implement me
	panic("implement me")
}

func (r *RedisQueueService) Subscribe(ctx context.Context, chanel string) *redis.PubSub {
	return r.client.Subscribe(ctx, chanel)
}

func (r *RedisQueueService) Publish(ctx context.Context, chanel string, data interface{}) error {
	resp := r.client.Publish(ctx, chanel, data)
	if resp.Err() != nil {
		return resp.Err()
	}
	return nil
}

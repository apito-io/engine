package redis

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/apito-io/engine/models"
	"github.com/redis/go-redis/v9"
)

// #todo redis sentinal service
//
// KV requires a Redis primary (master). Replicas are read-only and will return
// "READONLY You can't write against a read only replica" on Set/SetValue.
// Ensure KV_HOST (and QUEUE_HOST if shared) points to the primary, not a replica.

type KVRedisService struct {
	client *redis.Client
}

func GetKVRedisDriver(cfg *models.Config) (*KVRedisService, error) {

	dbNo, err := strconv.Atoi(cfg.KVStorageEngineDatabase)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.KVStorageEngineHost, cfg.KVStorageEnginePort),
		Password: cfg.KVStorageEnginePassword, // no password set
		DB:       dbNo,                        // use default DB
	})

	ctx := context.Background()

	err = client.Ping(ctx).Err()
	if err != nil {
		return nil, errors.New(fmt.Sprintf(`redis KV driver error : %s`, err))
	}

	return &KVRedisService{
		client: client,
	}, nil
}

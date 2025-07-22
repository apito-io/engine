package redis

import (
	"fmt"
	"strconv"

	"github.com/apito-io/engine/models"
	"github.com/redis/go-redis/v9"
)

// #todo redis sentinal service

type RedisQueueService struct {
	client *redis.Client
}

func GetRedisQueueDriver(cfg *models.Config) (*RedisQueueService, error) {

	dbNo, err := strconv.Atoi(cfg.KVStorageEngineDatabase)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.KVStorageEngineHost, cfg.KVStorageEnginePort),
		Password: cfg.KVStorageEnginePassword, // no password set
		DB:       dbNo,                        // use default DB
	})

	return &RedisQueueService{
		client: client,
	}, nil
}

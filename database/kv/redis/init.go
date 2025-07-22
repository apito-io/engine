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

type KVRedisService struct {
	client *redis.Client
}

func GetKVRedisDriver(ctx context.Context, cfg *models.Config) (*KVRedisService, error) {

	dbNo, err := strconv.Atoi(cfg.KVStorageEngineDatabase)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.KVStorageEngineHost, cfg.KVStorageEnginePort),
		Password: cfg.KVStorageEnginePassword, // no password set
		DB:       dbNo,                        // use default DB
	})

	err = client.Ping(ctx).Err()
	if err != nil {
		return nil, errors.New(fmt.Sprintf(`redis KV driver error : %s`, err))
	}

	return &KVRedisService{
		client: client,
	}, nil
}

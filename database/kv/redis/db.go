package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redis/go-redis/v9"
	"time"
)

func (r *KVRedisService) AddToSortedSets(ctx context.Context, setName string, key string, exp time.Duration) error {
	expiryTime := time.Now().Add(exp).Unix()
	err := r.client.ZAdd(ctx, setName, redis.Z{
		Score:  float64(expiryTime),
		Member: key,
	}).Err()
	return err
}

func (r *KVRedisService) GetFromSortedSets(ctx context.Context, setName string, key string) (float64, error) {
	_val, err := r.client.ZScore(ctx, setName, key).Result()
	if errors.Is(err, redis.Nil) {
		return -1, nil
	} else if err != nil {
		return -1, err
	}
	return _val, nil
}

func (r *KVRedisService) SetToHashMap(ctx context.Context, hash, key string, value string) error {

	err := r.client.HSet(ctx, hash, key, value).Err()
	if err != nil {
		msg := fmt.Sprintf("KVRedisService :: Connect :: SetValue :: Error :: %s", err.Error())
		return errors.New(msg)
	}
	return nil
}

func (r *KVRedisService) Subscribe(ctx context.Context, chanel string) *redis.PubSub {
	return r.client.Subscribe(ctx, chanel)
}

func (r *KVRedisService) Publish(ctx context.Context, chanel string, data interface{}) error {
	resp := r.client.Publish(ctx, chanel, data)
	if resp.Err() != nil {
		return resp.Err()
	}
	return nil
}

func (r *KVRedisService) GetFromHashMap(ctx context.Context, hash, key string) (string, error) {
	return r.client.HGet(ctx, hash, key).Result()
}

func (r *KVRedisService) CheckKeyHashMap(ctx context.Context, hash, key string) bool {
	return r.client.HExists(ctx, hash, key).Val()
}

func (r *KVRedisService) DelValue(ctx context.Context, key string) error {
	err := r.client.Del(ctx, key).Err()
	if err != nil {
		msg := fmt.Sprintf("KVRedisService :: Connect :: DelValue :: Error :: %s", err.Error())
		return errors.New(msg)
	}
	return nil
}

func (r *KVRedisService) SetValue(ctx context.Context, key string, value string, expiration time.Duration) error {

	err := r.client.Set(ctx, key, value, expiration).Err()
	if err != nil {
		msg := fmt.Sprintf("KVRedisService :: Connect :: SetValue :: Error :: %s", err.Error())
		return errors.New(msg)
	}
	return nil
}

func (r *KVRedisService) SetJSONObject(ctx context.Context, key string, value interface{}, expiration time.Duration) error {

	b, err := json.Marshal(value)
	if err != nil {
		return err
	}

	err = r.client.Set(ctx, key, string(b), expiration).Err()
	if err != nil {
		msg := fmt.Sprintf("KVRedisService :: Connect :: SetValue :: Error :: %s", err.Error())
		return errors.New(msg)
	}
	return nil
}

func (r *KVRedisService) GetJSONObject(ctx context.Context, key string) (interface{}, error) {
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var jsonData interface{}
	err = json.Unmarshal([]byte(data), &jsonData)
	if err != nil {
		return nil, err
	}
	return jsonData, nil

}

func (r *KVRedisService) CheckRedisKey(ctx context.Context, keys ...string) (bool, error) {
	// Check whether the user exists or not ?
	var key string
	switch len(keys) {
	case 2:
		key = fmt.Sprintf("%s:%s", keys[0], keys[1])
	default:
		key = keys[0]
	}

	_, err := r.GetValue(ctx, key)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, errors.New("nothing Found")
		}
		return false, err
	}
	return true, nil
}

func (r *KVRedisService) GetValue(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *KVRedisService) AddToSets(ctx context.Context, key string, value string) error {

	err := r.client.SAdd(ctx, key, value).Err()
	if err != nil {
		msg := fmt.Sprintf("KVRedisService :: Connect :: Set Add :: Error :: %s", err.Error())
		return errors.New(msg)
	}
	return nil
}

func (r *KVRedisService) RemoveSets(ctx context.Context, key string, value string) error {

	err := r.client.SRem(ctx, key, value).Err()
	if err != nil {
		msg := fmt.Sprintf("KVRedisService :: Connect :: Set Add :: Error :: %s", err.Error())
		return errors.New(msg)
	}
	return nil
}

func (r *KVRedisService) GetStoreDomains(ctx context.Context, sets string, member string) (bool, error) {
	return r.client.SIsMember(ctx, sets, member).Result()
}

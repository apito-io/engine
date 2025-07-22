package interfaces

import (
	"context"
	"time"
)

// KeyValueServiceInterface is an interface that defines the methods for interacting with Redis.
type KeyValueServiceInterface interface {
	// AddToSortedSets adds a key to a sorted set with a given TTL (Time To Live) in seconds.
	AddToSortedSets(ctx context.Context, setName string, key string, exp time.Duration) error

	// GetFromSortedSets retrieves a key from a sorted set.
	GetFromSortedSets(ctx context.Context, setName string, key string) (float64, error)

	// SetToHashMap sets a key-value pair in a hash.
	SetToHashMap(ctx context.Context, hash, key string, value string) error

	// GetFromHashMap retrieves a value from a hash using a key.
	GetFromHashMap(ctx context.Context, hash, key string) (string, error)

	// CheckKeyHashMap checks if a key exists in a hash.
	CheckKeyHashMap(ctx context.Context, hash, key string) bool

	// DelValue deletes a value using a key.
	DelValue(ctx context.Context, key string) error

	// SetValue sets a value with a key with a given expiration time.
	SetValue(ctx context.Context, key string, value string, expiration time.Duration) error

	// SetJSONObject sets a JSON object with a key with a given expiration time.
	SetJSONObject(ctx context.Context, key string, value interface{}, expiration time.Duration) error

	// GetJSONObject retrieves a JSON object using a key.
	GetJSONObject(ctx context.Context, key string) (interface{}, error)

	// CheckRedisKey checks if one or more keys exist.
	CheckRedisKey(ctx context.Context, keys ...string) (bool, error)

	// GetValue retrieves a value using a key.
	GetValue(ctx context.Context, key string) (string, error)

	// AddToSets adds a value to a set.
	AddToSets(ctx context.Context, key string, value string) error

	// RemoveSets removes a value from a set.
	RemoveSets(ctx context.Context, key string, value string) error
}

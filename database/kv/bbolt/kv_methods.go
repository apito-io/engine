package bbolt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	apitobolt "github.com/apito-io/apitoBolt"
)

// SortedSetItem represents an item in a sorted set
type SortedSetItem struct {
	ID     string  `json:"id"`
	SetKey string  `json:"set_key"`
	Score  float64 `json:"score"`
}

// ExpirationItem represents an expiration entry
type ExpirationItem struct {
	ID         string `json:"id"`
	Key        string `json:"key"`
	ExpireTime int64  `json:"expire_time"`
}

// AddToSortedSets adds a key to a sorted set with a given TTL (Time To Live) in seconds.
func (k *KVBoltService) AddToSortedSets(ctx context.Context, setName string, key string, exp time.Duration) error {
	return k.store.Update(func(tx *apitobolt.Tx) error {
		sortedSetsCol := tx.Collection("sorted_sets")

		// Create sorted set item
		itemID := fmt.Sprintf("%s:%s", setName, key)
		item := SortedSetItem{
			ID:     itemID,
			SetKey: setName,
			Score:  float64(time.Now().Unix()),
		}

		if _, err := sortedSetsCol.Save(&item); err != nil {
			return fmt.Errorf("failed to add key to sorted set: %w", err)
		}

		// Handle expiration if specified
		if exp > 0 {
			expCol := tx.Collection("expirations")
			expItem := ExpirationItem{
				ID:         itemID,
				Key:        key,
				ExpireTime: time.Now().Add(exp).Unix(),
			}

			if _, err := expCol.Save(&expItem); err != nil {
				return fmt.Errorf("failed to set expiration for key: %w", err)
			}
		}

		return nil
	})
}

// GetFromSortedSets retrieves a key from a sorted set.
func (k *KVBoltService) GetFromSortedSets(ctx context.Context, setName string, key string) (float64, error) {
	var item SortedSetItem
	itemID := fmt.Sprintf("%s:%s", setName, key)

	err := k.store.View(func(tx *apitobolt.Tx) error {
		sortedSetsCol := tx.Collection("sorted_sets")

		if err := sortedSetsCol.FindByID(itemID, &item); err != nil {
			return errors.New("key not found in sorted set")
		}

		// Check if expired
		expCol := tx.Collection("expirations")
		var expItem ExpirationItem
		if err := expCol.FindByID(itemID, &expItem); err == nil {
			if time.Now().Unix() > expItem.ExpireTime {
				return errors.New("key expired")
			}
		}

		return nil
	})

	return item.Score, err
}

// HashMapItem represents an item in a hash map
type HashMapItem struct {
	ID    string `json:"id"`
	Hash  string `json:"hash"`
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SetToHashMap sets a key-value pair in a hash.
func (k *KVBoltService) SetToHashMap(ctx context.Context, hash, key string, value string) error {
	return k.store.Update(func(tx *apitobolt.Tx) error {
		hashMapsCol := tx.Collection("hashmaps")

		itemID := fmt.Sprintf("%s:%s", hash, key)
		item := HashMapItem{
			ID:    itemID,
			Hash:  hash,
			Key:   key,
			Value: value,
		}

		if _, err := hashMapsCol.Save(&item); err != nil {
			return fmt.Errorf("failed to set hash value: %w", err)
		}

		return nil
	})
}

// GetFromHashMap retrieves a value from a hash using a key.
func (k *KVBoltService) GetFromHashMap(ctx context.Context, hash, key string) (string, error) {
	var item HashMapItem
	itemID := fmt.Sprintf("%s:%s", hash, key)

	err := k.store.View(func(tx *apitobolt.Tx) error {
		hashMapsCol := tx.Collection("hashmaps")

		if err := hashMapsCol.FindByID(itemID, &item); err != nil {
			return errors.New("key not found in hash")
		}

		return nil
	})

	return item.Value, err
}

// CheckKeyHashMap checks if a key exists in a hash.
func (k *KVBoltService) CheckKeyHashMap(ctx context.Context, hash, key string) bool {
	var item HashMapItem
	itemID := fmt.Sprintf("%s:%s", hash, key)

	err := k.store.View(func(tx *apitobolt.Tx) error {
		hashMapsCol := tx.Collection("hashmaps")
		return hashMapsCol.FindByID(itemID, &item)
	})

	return err == nil
}

// DataItem represents a simple key-value data item
type DataItem struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// DelValue deletes a value using a key.
func (k *KVBoltService) DelValue(ctx context.Context, key string) error {
	return k.store.Update(func(tx *apitobolt.Tx) error {
		dataCol := tx.Collection("data")

		item := DataItem{ID: key}
		if err := dataCol.DeleteStruct(&item); err != nil {
			return fmt.Errorf("failed to delete key: %w", err)
		}

		return nil
	})
}

// SetValue sets a value with a key with a given expiration time.
func (k *KVBoltService) SetValue(ctx context.Context, key string, value string, expiration time.Duration) error {
	return k.store.Update(func(tx *apitobolt.Tx) error {
		dataCol := tx.Collection("data")

		item := DataItem{
			ID:    key,
			Value: value,
		}

		if _, err := dataCol.Save(&item); err != nil {
			return fmt.Errorf("failed to set value: %w", err)
		}

		// Handle expiration if specified
		if expiration > 0 {
			expCol := tx.Collection("expirations")
			expItem := ExpirationItem{
				ID:         key,
				Key:        key,
				ExpireTime: time.Now().Add(expiration).Unix(),
			}

			if _, err := expCol.Save(&expItem); err != nil {
				return fmt.Errorf("failed to set expiration: %w", err)
			}
		}

		return nil
	})
}

// SetJSONObject sets a JSON object with a key with a given expiration time.
func (k *KVBoltService) SetJSONObject(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return k.SetValue(ctx, key, string(jsonData), expiration)
}

// GetJSONObject retrieves a JSON object using a key.
func (k *KVBoltService) GetJSONObject(ctx context.Context, key string) (interface{}, error) {
	jsonStr, err := k.GetValue(ctx, key)
	if err != nil {
		return nil, err
	}

	var result interface{}
	err = json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	return result, nil
}

// CheckRedisKey checks if one or more keys exist.
func (k *KVBoltService) CheckRedisKey(ctx context.Context, keys ...string) (bool, error) {
	var allExist bool
	err := k.store.View(func(tx *apitobolt.Tx) error {
		dataCol := tx.Collection("data")
		expCol := tx.Collection("expirations")

		for _, key := range keys {
			var item DataItem
			if err := dataCol.FindByID(key, &item); err != nil {
				allExist = false
				return nil
			}

			// Check if expired
			var expItem ExpirationItem
			if err := expCol.FindByID(key, &expItem); err == nil {
				if time.Now().Unix() > expItem.ExpireTime {
					allExist = false
					return nil
				}
			}
		}

		allExist = true
		return nil
	})

	return allExist, err
}

// GetValue retrieves a value using a key.
func (k *KVBoltService) GetValue(ctx context.Context, key string) (string, error) {
	var item DataItem
	err := k.store.View(func(tx *apitobolt.Tx) error {
		dataCol := tx.Collection("data")

		if err := dataCol.FindByID(key, &item); err != nil {
			return errors.New("key not found")
		}

		// Check if expired
		expCol := tx.Collection("expirations")
		var expItem ExpirationItem
		if err := expCol.FindByID(key, &expItem); err == nil {
			if time.Now().Unix() > expItem.ExpireTime {
				return errors.New("key expired")
			}
		}

		return nil
	})

	return item.Value, err
}

// SetItem represents an item in a set
type SetItem struct {
	ID     string `json:"id"`
	SetKey string `json:"set_key"`
	Value  string `json:"value"`
}

// AddToSets adds a value to a set.
func (k *KVBoltService) AddToSets(ctx context.Context, key string, value string) error {
	return k.store.Update(func(tx *apitobolt.Tx) error {
		setsCol := tx.Collection("sets")

		itemID := fmt.Sprintf("%s:%s", key, value)
		item := SetItem{
			ID:     itemID,
			SetKey: key,
			Value:  value,
		}

		if _, err := setsCol.Save(&item); err != nil {
			return fmt.Errorf("failed to add value to set: %w", err)
		}

		return nil
	})
}

// RemoveSets removes a value from a set.
func (k *KVBoltService) RemoveSets(ctx context.Context, key string, value string) error {
	return k.store.Update(func(tx *apitobolt.Tx) error {
		setsCol := tx.Collection("sets")

		itemID := fmt.Sprintf("%s:%s", key, value)
		item := SetItem{ID: itemID}

		if err := setsCol.DeleteStruct(&item); err != nil {
			// Set doesn't exist, nothing to remove - not an error
			return nil
		}

		return nil
	})
}

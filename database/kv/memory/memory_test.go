package memory

import (
	"context"
	"github.com/apito-io/engine/models"
	"testing"
	"time"
)

func TestKVMemoryService_BasicOperations(t *testing.T) {
	cfg := &models.Config{}
	service, err := GetKVMemoryDriver(cfg)
	if err != nil {
		t.Fatalf("Failed to create memory KV service: %v", err)
	}
	defer service.Close()

	ctx := context.Background()

	// Test SetValue and GetValue
	t.Run("SetValue and GetValue", func(t *testing.T) {
		key := "test_key"
		value := "test_value"

		err := service.SetValue(ctx, key, value, 0)
		if err != nil {
			t.Errorf("SetValue failed: %v", err)
		}

		retrievedValue, err := service.GetValue(ctx, key)
		if err != nil {
			t.Errorf("GetValue failed: %v", err)
		}

		if retrievedValue != value {
			t.Errorf("Expected %s, got %s", value, retrievedValue)
		}
	})

	// Test expiration
	t.Run("Expiration", func(t *testing.T) {
		key := "expire_key"
		value := "expire_value"

		err := service.SetValue(ctx, key, value, 100*time.Millisecond)
		if err != nil {
			t.Errorf("SetValue failed: %v", err)
		}

		// Should exist immediately
		_, err = service.GetValue(ctx, key)
		if err != nil {
			t.Errorf("GetValue failed before expiration: %v", err)
		}

		// Wait for expiration
		time.Sleep(150 * time.Millisecond)

		// Should be expired
		_, err = service.GetValue(ctx, key)
		if err == nil {
			t.Error("Expected error for expired key")
		}
	})

	// Test JSON operations
	t.Run("JSON Operations", func(t *testing.T) {
		key := "json_key"
		data := map[string]interface{}{
			"name": "test",
			"age":  25,
		}

		err := service.SetJSONObject(ctx, key, data, 0)
		if err != nil {
			t.Errorf("SetJSONObject failed: %v", err)
		}

		retrievedData, err := service.GetJSONObject(ctx, key)
		if err != nil {
			t.Errorf("GetJSONObject failed: %v", err)
		}

		dataMap := retrievedData.(map[string]interface{})
		if dataMap["name"] != "test" {
			t.Errorf("Expected name=test, got %v", dataMap["name"])
		}
	})

	// Test HashMap operations
	t.Run("HashMap Operations", func(t *testing.T) {
		hash := "test_hash"
		key := "hash_key"
		value := "hash_value"

		err := service.SetToHashMap(ctx, hash, key, value)
		if err != nil {
			t.Errorf("SetToHashMap failed: %v", err)
		}

		exists := service.CheckKeyHashMap(ctx, hash, key)
		if !exists {
			t.Error("Expected key to exist in hash")
		}

		retrievedValue, err := service.GetFromHashMap(ctx, hash, key)
		if err != nil {
			t.Errorf("GetFromHashMap failed: %v", err)
		}

		if retrievedValue != value {
			t.Errorf("Expected %s, got %s", value, retrievedValue)
		}
	})

	// Test Set operations
	t.Run("Set Operations", func(t *testing.T) {
		setKey := "test_set"
		value1 := "value1"
		value2 := "value2"

		err := service.AddToSets(ctx, setKey, value1)
		if err != nil {
			t.Errorf("AddToSets failed: %v", err)
		}

		err = service.AddToSets(ctx, setKey, value2)
		if err != nil {
			t.Errorf("AddToSets failed: %v", err)
		}

		// Test GetStoreDomains
		exists, err := service.GetStoreDomains(ctx, setKey, value1)
		if err != nil {
			t.Errorf("GetStoreDomains failed: %v", err)
		}
		if !exists {
			t.Error("Expected value1 to exist in set")
		}

		// Test with non-existent value
		exists, err = service.GetStoreDomains(ctx, setKey, "nonexistent")
		if err != nil {
			t.Errorf("GetStoreDomains failed: %v", err)
		}
		if exists {
			t.Error("Expected nonexistent value to not exist in set")
		}

		err = service.RemoveSets(ctx, setKey, value1)
		if err != nil {
			t.Errorf("RemoveSets failed: %v", err)
		}

		// Verify removal
		exists, err = service.GetStoreDomains(ctx, setKey, value1)
		if err != nil {
			t.Errorf("GetStoreDomains failed: %v", err)
		}
		if exists {
			t.Error("Expected value1 to not exist after removal")
		}
	})

	// Test Sorted Set operations
	t.Run("Sorted Set Operations", func(t *testing.T) {
		setName := "test_sorted_set"
		key := "sorted_key"

		err := service.AddToSortedSets(ctx, setName, key, 0)
		if err != nil {
			t.Errorf("AddToSortedSets failed: %v", err)
		}

		score, err := service.GetFromSortedSets(ctx, setName, key)
		if err != nil {
			t.Errorf("GetFromSortedSets failed: %v", err)
		}

		if score <= 0 {
			t.Error("Expected positive score")
		}
	})

	// Test CheckRedisKey
	t.Run("CheckRedisKey", func(t *testing.T) {
		key1 := "check_key1"
		key2 := "check_key2"
		key3 := "nonexistent_key"

		service.SetValue(ctx, key1, "value1", 0)
		service.SetValue(ctx, key2, "value2", 0)

		// Check existing keys
		exists, err := service.CheckRedisKey(ctx, key1, key2)
		if err != nil {
			t.Errorf("CheckRedisKey failed: %v", err)
		}
		if !exists {
			t.Error("Expected keys to exist")
		}

		// Check with non-existent key
		exists, err = service.CheckRedisKey(ctx, key1, key3)
		if err != nil {
			t.Errorf("CheckRedisKey failed: %v", err)
		}
		if exists {
			t.Error("Expected false when one key doesn't exist")
		}
	})

	// Test DelValue
	t.Run("DelValue", func(t *testing.T) {
		key := "delete_key"
		value := "delete_value"

		service.SetValue(ctx, key, value, 0)

		err := service.DelValue(ctx, key)
		if err != nil {
			t.Errorf("DelValue failed: %v", err)
		}

		_, err = service.GetValue(ctx, key)
		if err == nil {
			t.Error("Expected error after deleting key")
		}
	})
}

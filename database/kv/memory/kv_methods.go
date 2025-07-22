package memory

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

// AddToSortedSets adds a key to a sorted set with a given TTL (Time To Live) in seconds.
func (m *KVMemoryService) AddToSortedSets(ctx context.Context, setName string, key string, exp time.Duration) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.sortedSets[setName] == nil {
		m.sortedSets[setName] = make(map[string]float64)
	}

	// Use current timestamp as score
	score := float64(time.Now().Unix())
	m.sortedSets[setName][key] = score

	// Handle expiration by setting a timer
	if exp > 0 {
		go func() {
			time.Sleep(exp)
			m.mutex.Lock()
			defer m.mutex.Unlock()
			if m.sortedSets[setName] != nil {
				delete(m.sortedSets[setName], key)
				if len(m.sortedSets[setName]) == 0 {
					delete(m.sortedSets, setName)
				}
			}
		}()
	}

	return nil
}

// GetFromSortedSets retrieves a key from a sorted set.
func (m *KVMemoryService) GetFromSortedSets(ctx context.Context, setName string, key string) (float64, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.sortedSets[setName] == nil {
		return 0, errors.New("sorted set not found")
	}

	score, exists := m.sortedSets[setName][key]
	if !exists {
		return 0, errors.New("key not found in sorted set")
	}

	return score, nil
}

// SetToHashMap sets a key-value pair in a hash.
func (m *KVMemoryService) SetToHashMap(ctx context.Context, hash, key string, value string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.hashMaps[hash] == nil {
		m.hashMaps[hash] = make(map[string]string)
	}

	m.hashMaps[hash][key] = value
	return nil
}

// GetFromHashMap retrieves a value from a hash using a key.
func (m *KVMemoryService) GetFromHashMap(ctx context.Context, hash, key string) (string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.hashMaps[hash] == nil {
		return "", errors.New("hash not found")
	}

	value, exists := m.hashMaps[hash][key]
	if !exists {
		return "", errors.New("key not found in hash")
	}

	return value, nil
}

// CheckKeyHashMap checks if a key exists in a hash.
func (m *KVMemoryService) CheckKeyHashMap(ctx context.Context, hash, key string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.hashMaps[hash] == nil {
		return false
	}

	_, exists := m.hashMaps[hash][key]
	return exists
}

// DelValue deletes a value using a key.
func (m *KVMemoryService) DelValue(ctx context.Context, key string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	delete(m.data, key)
	return nil
}

// SetValue sets a value with a key with a given expiration time.
func (m *KVMemoryService) SetValue(ctx context.Context, key string, value string, expiration time.Duration) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	item := &memoryItem{
		value: value,
	}

	if expiration > 0 {
		item.expiration = time.Now().Add(expiration)
		item.hasExp = true
	}

	m.data[key] = item
	return nil
}

// SetJSONObject sets a JSON object with a key with a given expiration time.
func (m *KVMemoryService) SetJSONObject(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return m.SetValue(ctx, key, string(jsonData), expiration)
}

// GetJSONObject retrieves a JSON object using a key.
func (m *KVMemoryService) GetJSONObject(ctx context.Context, key string) (interface{}, error) {
	jsonStr, err := m.GetValue(ctx, key)
	if err != nil {
		return nil, err
	}

	var result interface{}
	err = json.Unmarshal([]byte(jsonStr), &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// CheckRedisKey checks if one or more keys exist.
func (m *KVMemoryService) CheckRedisKey(ctx context.Context, keys ...string) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	for _, key := range keys {
		item, exists := m.data[key]
		if !exists {
			return false, nil
		}
		if m.isExpired(item) {
			return false, nil
		}
	}

	return true, nil
}

// GetValue retrieves a value using a key.
func (m *KVMemoryService) GetValue(ctx context.Context, key string) (string, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	item, exists := m.data[key]
	if !exists {
		return "", errors.New("key not found")
	}

	if m.isExpired(item) {
		// Remove expired item
		m.mutex.RUnlock()
		m.mutex.Lock()
		delete(m.data, key)
		m.mutex.Unlock()
		m.mutex.RLock()
		return "", errors.New("key expired")
	}

	return item.value, nil
}

// AddToSets adds a value to a set.
func (m *KVMemoryService) AddToSets(ctx context.Context, key string, value string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.sets[key] == nil {
		m.sets[key] = make(map[string]bool)
	}

	m.sets[key][value] = true
	return nil
}

// RemoveSets removes a value from a set.
func (m *KVMemoryService) RemoveSets(ctx context.Context, key string, value string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.sets[key] == nil {
		return nil // Set doesn't exist, nothing to remove
	}

	delete(m.sets[key], value)

	// Clean up empty set
	if len(m.sets[key]) == 0 {
		delete(m.sets, key)
	}

	return nil
}

// GetStoreDomains checks if a member exists in a set (equivalent to Redis SISMEMBER)
func (m *KVMemoryService) GetStoreDomains(ctx context.Context, sets string, member string) (bool, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if m.sets[sets] == nil {
		return false, nil
	}

	exists := m.sets[sets][member]
	return exists, nil
}

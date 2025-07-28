package memory

import (
	"sync"
	"time"

	"github.com/apito-io/engine/models"
)

// KVMemoryService implements KeyValueServiceInterface using in-memory storage
type KVMemoryService struct {
	// Regular key-value store
	data map[string]*memoryItem
	// Hash maps
	hashMaps map[string]map[string]string
	// Sets
	sets map[string]map[string]bool
	// Sorted sets (using timestamp as score)
	sortedSets map[string]map[string]float64
	// Sorted sets expiration tracking
	expirations map[string]map[string]time.Time
	// Mutex for thread safety
	mutex sync.RWMutex
	// Cleanup ticker
	cleanupTicker *time.Ticker
	cleanupDone   chan bool
}

type memoryItem struct {
	value      string
	expiration time.Time
	hasExp     bool
}

func GetKVMemoryDriver(cfg *models.Config) (*KVMemoryService, error) {
	service := &KVMemoryService{
		data:          make(map[string]*memoryItem),
		hashMaps:      make(map[string]map[string]string),
		sets:          make(map[string]map[string]bool),
		sortedSets:    make(map[string]map[string]float64),
		expirations:   make(map[string]map[string]time.Time),
		cleanupTicker: time.NewTicker(1 * time.Minute), // Cleanup every minute
		cleanupDone:   make(chan bool),
	}

	// Start cleanup goroutine
	go service.cleanupExpired()

	return service, nil
}

// cleanupExpired removes expired items
func (m *KVMemoryService) cleanupExpired() {
	for {
		select {
		case <-m.cleanupTicker.C:
			m.mutex.Lock()
			now := time.Now()

			// Clean up regular key-value items
			for key, item := range m.data {
				if item.hasExp && now.After(item.expiration) {
					delete(m.data, key)
				}
			}

			// Clean up expired sorted set items
			for setName, expMap := range m.expirations {
				for key, expTime := range expMap {
					if now.After(expTime) {
						if m.sortedSets[setName] != nil {
							delete(m.sortedSets[setName], key)
							if len(m.sortedSets[setName]) == 0 {
								delete(m.sortedSets, setName)
							}
						}
						delete(expMap, key)
					}
				}
				if len(expMap) == 0 {
					delete(m.expirations, setName)
				}
			}

			m.mutex.Unlock()
		case <-m.cleanupDone:
			return
		}
	}
}

// Close stops the cleanup goroutine
func (m *KVMemoryService) Close() {
	m.cleanupTicker.Stop()
	close(m.cleanupDone)
}

// isExpired checks if an item has expired
func (m *KVMemoryService) isExpired(item *memoryItem) bool {
	if !item.hasExp {
		return false
	}
	return time.Now().After(item.expiration)
}

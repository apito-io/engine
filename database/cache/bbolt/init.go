package bbolt

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	apitobolt "github.com/apito-io/apitoBolt"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// CacheDriver implements CacheDBInterface using ApitoBolt
type CacheDriver struct {
	store        *apitobolt.Store
	dbPath       string
	ProjectCache sync.Map
}

// Ensure CacheDriver implements CacheDBInterface
var _ interfaces.CacheDBInterface = (*CacheDriver)(nil)

func GetBoltCacheDriver(cfg *models.Config) (*CacheDriver, error) {
	// Determine database path
	dbPath := filepath.Join(cfg.DefaultDatabaseDir, cfg.CacheDBName)

	// Expand path (handles ~ and converts to absolute path)
	var err error
	dbPath, err = utility.ExpandPath(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand database path %s: %v", dbPath, err)
	}

	// Create directory if it doesn't exist
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
	}

	log.Printf("Opening Cache BBolt database at: %s", dbPath)

	// Open BBolt database using ApitoBolt
	store, err := apitobolt.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt database: %w", err)
	}

	// Initialize collections
	collections := []string{"projects", "app_cache", "cache", "expirations"}
	for _, collectionName := range collections {
		collection := store.Collection(collectionName)
		if err := collection.Init(); err != nil {
			store.Close()
			return nil, fmt.Errorf("failed to initialize collection %s: %w", collectionName, err)
		}
	}

	return &CacheDriver{
		store:        store,
		dbPath:       dbPath,
		ProjectCache: sync.Map{},
	}, nil
}

// Close closes the BoltDB connection
func (c *CacheDriver) Close() error {
	return c.store.Close()
}

// parseTTL parses the TTL from config, similar to memory cache driver
func (c *CacheDriver) parseTTL() time.Duration {
	// Default to 600 seconds if not set or invalid
	return 600 * time.Second
}

// CacheExpiration represents an expiration entry
type CacheExpiration struct {
	ID         string `json:"id"`
	ExpireTime int64  `json:"expire_time"`
}

// isExpired checks if a key has expired based on stored expiration time
func (c *CacheDriver) isExpired(ctx context.Context, key string) (bool, error) {
	var expItem CacheExpiration
	err := c.store.View(func(tx *apitobolt.Tx) error {
		expCol := tx.Collection("expirations")
		return expCol.FindByID(key, &expItem)
	})

	if err != nil {
		// No expiration set, not expired
		return false, nil
	}

	return time.Now().Unix() > expItem.ExpireTime, nil
}

// setExpiration sets expiration time for a key
func (c *CacheDriver) setExpiration(ctx context.Context, key string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil // No expiration
	}

	return c.store.Update(func(tx *apitobolt.Tx) error {
		expCol := tx.Collection("expirations")
		expItem := CacheExpiration{
			ID:         key,
			ExpireTime: time.Now().Add(ttl).Unix(),
		}

		_, err := expCol.Save(&expItem)
		return err
	})
}

// removeExpiration removes expiration time for a key
func (c *CacheDriver) removeExpiration(ctx context.Context, key string) error {
	return c.store.Update(func(tx *apitobolt.Tx) error {
		expCol := tx.Collection("expirations")
		item := CacheExpiration{ID: key}
		// Don't return error if item doesn't exist
		_ = expCol.DeleteStruct(&item)
		return nil
	})
}

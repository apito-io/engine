package bbolt

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	apitobolt "github.com/apito-io/apitoBolt"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// KVBoltService implements KeyValueServiceInterface using ApitoBolt
type KVBoltService struct {
	store  *apitobolt.Store
	dbPath string
}

// Ensure KVBoltService implements KeyValueServiceInterface
var _ interfaces.KeyValueServiceInterface = (*KVBoltService)(nil)

func GetKVBoltDriver(cfg *models.Config) (*KVBoltService, error) {
	// Determine database path
	var dbPath string
	if cfg.KVStorageEngineDatabase != "" {
		dbPath = cfg.KVStorageEngineDatabase
	} else {
		// Expand home directory and create default path
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		dbPath = filepath.Join(homeDir, ".apito", "engine-data", "apito_kv.db")
	}

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

	log.Printf("Opening KV BBolt database at: %s", dbPath)

	// Open BBolt database using ApitoBolt
	store, err := apitobolt.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt database: %w", err)
	}

	// Initialize collections
	collections := []string{"data", "hashmaps", "sets", "sorted_sets", "expirations"}
	for _, collectionName := range collections {
		collection := store.Collection(collectionName)
		if err := collection.Init(); err != nil {
			store.Close()
			return nil, fmt.Errorf("failed to initialize collection %s: %w", collectionName, err)
		}
	}

	return &KVBoltService{
		store:  store,
		dbPath: dbPath,
	}, nil
}

// Close closes the BoltDB connection
func (k *KVBoltService) Close() error {
	return k.store.Close()
}

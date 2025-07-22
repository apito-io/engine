package badger

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/dgraph-io/badger/v4"
)

type BadgerDriver struct {
	DB               *badger.DB
	DriverCredential *models.DriverCredentials
}

// GetBadgerDriver creates a new BadgerDB project driver instance
func GetBadgerDriver(driverCredentials *models.DriverCredentials) (*BadgerDriver, error) {
	// BadgerDB uses the Database field as the directory path
	dbPath := driverCredentials.Database
	if dbPath == "" {
		dbPath = "./badger_project_db"
	}

	// Ensure the path exists
	opts := badger.DefaultOptions(dbPath).WithLogger(nil)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %v", err)
	}

	return &BadgerDriver{
		DB:               db,
		DriverCredential: driverCredentials,
	}, nil
}

// Close closes the BadgerDB database connection
func (b *BadgerDriver) Close() error {
	if b.DB != nil {
		return b.DB.Close()
	}
	return nil
}

// DeleteProject deletes a project and all related data
func (b *BadgerDriver) DeleteProject(ctx context.Context, projectID string) error {
	return b.DB.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		var keysToDelete [][]byte

		// Find all keys related to this project
		prefixes := []string{
			fmt.Sprintf("project_doc:%s:", projectID),
			fmt.Sprintf("project_rel:%s:", projectID),
			fmt.Sprintf("project_rev:%s:", projectID),
			fmt.Sprintf("project_builder:%s:", projectID),
			fmt.Sprintf("project_user:%s:", projectID),
			fmt.Sprintf("project_model:%s:", projectID),
		}

		for _, prefix := range prefixes {
			prefixBytes := []byte(prefix)
			for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
				key := it.Item().Key()
				keysToDelete = append(keysToDelete, append([]byte(nil), key...))
			}
		}

		// Delete all collected keys
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return err
			}
		}

		return nil
	})
}

// TransferProject transfers a project from one user to another
func (b *BadgerDriver) TransferProject(ctx context.Context, userId, from, to string) error {
	return b.DB.Update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		// Find all documents for this project and update ownership
		prefix := []byte(fmt.Sprintf("project_doc:%s:", userId))
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := item.Key()

			err := item.Value(func(val []byte) error {
				var doc map[string]interface{}
				if err := json.Unmarshal(val, &doc); err != nil {
					return err
				}

				// Check if this document belongs to the 'from' user
				if ownerId, ok := doc["owner_id"]; ok && ownerId == from {
					doc["owner_id"] = to

					updatedData, err := json.Marshal(doc)
					if err != nil {
						return err
					}

					return txn.Set(key, updatedData)
				}

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})
}

// generateKey generates a BadgerDB key for different data types
func (b *BadgerDriver) generateKey(keyType, projectID string, parts ...string) string {
	allParts := append([]string{keyType, projectID}, parts...)
	return strings.Join(allParts, ":")
}

// parseKey parses a BadgerDB key and extracts its components
func (b *BadgerDriver) parseKey(key string) (keyType, projectID string, parts []string) {
	components := strings.Split(key, ":")
	if len(components) >= 2 {
		keyType = components[0]
		projectID = components[1]
		if len(components) > 2 {
			parts = components[2:]
		}
	}
	return
}

// keyExists checks if a key exists in BadgerDB
func (b *BadgerDriver) keyExists(key string) (bool, error) {
	var exists bool
	err := b.DB.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(key))
		if err == badger.ErrKeyNotFound {
			exists = false
			return nil
		} else if err != nil {
			return err
		}
		exists = true
		return nil
	})
	return exists, err
}

// getValue gets a value from BadgerDB
func (b *BadgerDriver) getValue(key string) ([]byte, error) {
	var value []byte
	err := b.DB.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		return item.Value(func(val []byte) error {
			value = append([]byte(nil), val...)
			return nil
		})
	})
	return value, err
}

// setValue sets a value in BadgerDB
func (b *BadgerDriver) setValue(key string, value []byte) error {
	return b.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
}

// deleteKey deletes a key from BadgerDB
func (b *BadgerDriver) deleteKey(key string) error {
	return b.DB.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// getAllWithPrefix gets all key-value pairs with a given prefix
func (b *BadgerDriver) getAllWithPrefix(prefix string) (map[string][]byte, error) {
	result := make(map[string][]byte)

	err := b.DB.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefixBytes := []byte(prefix)
		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			item := it.Item()
			key := string(item.Key())

			err := item.Value(func(val []byte) error {
				result[key] = append([]byte(nil), val...)
				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return result, err
}

// initializeMetadata creates initial metadata for a project
func (b *BadgerDriver) initializeMetadata(ctx context.Context, projectID string) error {
	metaKey := b.generateKey("project_meta", projectID)
	exists, err := b.keyExists(metaKey)
	if err != nil {
		return err
	}

	if !exists {
		metadata := map[string]interface{}{
			"project_id": projectID,
			"created_at": time.Now().Format(time.RFC3339),
			"version":    "1.0",
		}

		metaJSON, err := json.Marshal(metadata)
		if err != nil {
			return err
		}

		return b.setValue(metaKey, metaJSON)
	}

	return nil
}

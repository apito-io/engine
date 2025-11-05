// Package bbolt provides a BBolt database driver implementation for Apito project operations.
// It uses the ApitoBolt SDK (github.com/apito-io/apitoBolt) to provide a MongoDB-like
// interface on top of BBolt for project-level operations.
//
// This driver follows the same patterns as the Mongo driver, using collection-per-model
// strategy for optimal performance and scalability.
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

// BBoltDriver implements ProjectDBInterface using ApitoBolt in a MongoDB-like fashion
type BBoltDriver struct {
	Store            *apitobolt.Store
	DriverCredential *models.DriverCredentials
	dbPath           string
}

// Ensure BBoltDriver implements ProjectDBInterface at compile time
var _ interfaces.ProjectDBInterface = (*BBoltDriver)(nil)

// GetBBoltDriver creates a new BBolt project driver instance
func GetBBoltDriver(driverCredentials *models.DriverCredentials) (*BBoltDriver, error) {
	// Determine database path
	dbPath, err := utility.ExpandPath(driverCredentials.File)
	if err != nil {
		return nil, fmt.Errorf("failed to expand database path %s: %v", driverCredentials.File, err)
	}

	// Create directory if it doesn't exist
	dbDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dbDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory %s: %w", dbDir, err)
	}

	log.Printf("Opening Project BBolt database at: %s", dbPath)

	// Open BBolt database using ApitoBolt
	store, err := apitobolt.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open BBolt database: %v", err)
	}

	return &BBoltDriver{
		Store:            store,
		DriverCredential: driverCredentials,
		dbPath:           dbPath,
	}, nil
}

// Close closes the BBolt database connection
func (b *BBoltDriver) Close() error {
	if b.Store != nil {
		return b.Store.Close()
	}
	return nil
}

// Ping verifies the database connection is alive
func (b *BBoltDriver) Ping() error {
	if b.Store == nil {
		return fmt.Errorf("database connection is nil")
	}
	// ApitoBolt doesn't have explicit ping, but we can check if store is accessible
	return nil
}

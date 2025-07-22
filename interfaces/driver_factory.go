package interfaces

import (
	"github.com/apito-io/engine/models"
)

// DatabaseDriverFactory interface for creating database drivers
// This allows pro version to inject its own drivers while keeping core intact
type DatabaseDriverFactory interface {
	// CreateSystemDriver creates a system database driver
	CreateSystemDriver(conf *models.Config, engineConfig *models.DriverCredentials) (ApitoSystemDB, error)

	// CreateProjectDriver creates a project database driver
	CreateProjectDriver(conf *models.Config, engineConfig *models.DriverCredentials) (ProjectDBInterface, error)

	// SupportsEngine returns true if this factory can create drivers for the given engine
	SupportsEngine(engine string) bool
}

package interfaces

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
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

// ProProjectDriverFactory is implemented by factories that need pro-only credentials (e.g. Firestore service account JSON) when creating project DB drivers.
type ProProjectDriverFactory interface {
	DatabaseDriverFactory
	CreateProjectDriverWithProExtras(conf *models.Config, engineConfig *models.DriverCredentials, proExtras interface{}) (ProjectDBInterface, error)
}

// GraphQLServerFactory interface for creating GraphQL servers
// This allows pro version to inject its own enhanced GraphQL server while keeping core intact
type GraphQLServerFactory interface {
	// SupportsVersion returns true if this factory can create servers for the given version/edition
	SupportsVersion(version string) bool

	// CreateGraphQLServer creates a GraphQL server instance
	CreateGraphQLServer(ctx context.Context, cfg *models.Config, extensionRouter *echo.Group, mainEcho *echo.Echo) (GraphQLServerInterface, error)
}

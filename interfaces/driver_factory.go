package interfaces

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

// DatabaseDriverFactory interface for creating database drivers.
// A host binary may inject its own drivers while keeping this module intact.
type DatabaseDriverFactory interface {
	// CreateSystemDriver creates a system database driver
	CreateSystemDriver(conf *models.Config, engineConfig *models.DriverCredentials) (ApitoSystemDB, error)

	// CreateProjectDriver creates a project database driver
	CreateProjectDriver(conf *models.Config, engineConfig *models.DriverCredentials) (ProjectDBInterface, error)

	// SupportsEngine returns true if this factory can create drivers for the given engine
	SupportsEngine(engine string) bool
}

// ProjectDriverExtrasFactory is implemented by factories that accept extra host
// credentials when creating project database drivers.
type ProjectDriverExtrasFactory interface {
	DatabaseDriverFactory
	CreateProjectDriverWithExtras(conf *models.Config, engineConfig *models.DriverCredentials, extras interface{}) (ProjectDBInterface, error)
}

// GraphQLServerFactory interface for creating GraphQL servers.
// A host binary may inject its own server while keeping this module intact.
type GraphQLServerFactory interface {
	// SupportsVersion returns true if this factory can create servers for the given version/edition
	SupportsVersion(version string) bool

	// CreateGraphQLServer creates a GraphQL server instance
	CreateGraphQLServer(ctx context.Context, cfg *models.Config, extensionRouter *echo.Group, mainEcho *echo.Echo) (GraphQLServerInterface, error)
}

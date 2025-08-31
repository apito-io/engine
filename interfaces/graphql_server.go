// Package interfaces defines the contracts for all components in the Apito Engine.
// This allows for dependency injection and modular architecture between core and pro versions.
package interfaces

import (
	"bytes"
	"context"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
)

// GraphQLServerInterface defines the contract for GraphQL server implementations
// This allows pro version to extend core functionality while maintaining compatibility
type GraphQLServerInterface interface {
	// Authentication middleware
	Authorize() echo.MiddlewareFunc
	PublicFunctionRouteAuthorize() echo.MiddlewareFunc

	// Application cache and project management
	GetApplicationCache(router echo.Context) (*models.ApplicationCache, error)
	LoadProjectCache(ctx context.Context, projectID string) (*models.Project, error)
	UpdateApplicationCache(ctx context.Context, projectID string)

	// GraphQL field caching
	GetCacheGraphQLFieldsGeneration(ctx context.Context, projectID string, modelName string) (*QueryBuilderInformation, error)
	CacheGraphQLFieldsGeneration(ctx context.Context, projectID string, modelName string, val interface{}) error
	ExpireGraphQLFieldCache(ctx context.Context, projectID string, modelName string) error
	ExpireGraphQLProjectCache(ctx context.Context, projectID string) error

	// System messaging and subscriptions
	PublishSystemMessage(ctx context.Context, userID string, data *models.SubscriptionEvent) error

	// Provider management
	GetFunctionProvider() ([]string, error)
	GetStorageProvider() ([]string, error)

	// File handling
	GatherFileInfo(image []byte) (*models.FileDetails, error)
	PrepareFileInfo(router echo.Context, projectID string) (*models.FileDetails, *bytes.Buffer, error)

	// Plugin management
	LoadPlugins(ctx context.Context) error
	WaitForPluginsToLoad()
	PrintAllPluginRoutes()

	// System parameters
	BuildSystemParam(i echo.Context, project *models.Project) (*models.CommonSystemParams, error)
	NewParam(_param *models.CommonSystemParams) *models.CommonSystemParams

	// Schema and query building
	BuildServerQueriesAndMutations()
	LoadProjectSpecificPlugins(ctx context.Context, cache *models.ApplicationCache) error

	// Plugin monitoring (needed by router)
	GetHashiCorpPluginIDs() []string
	SetPluginMonitor(monitor interface{})
	GetConcreteServer() interface{} // For controller compatibility
}

type QueryBuilderInformation struct {
	DataObjects       graphql.Fields
	AggregateObjects  graphql.Fields
	WhereParamObjects graphql.InputObjectConfigFieldMap
	SortParamObjects  graphql.InputObjectConfigFieldMap
}

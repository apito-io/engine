package interfaces

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

// PluginRuntime is the callback surface a PluginHost needs from the GraphQL server.
type PluginRuntime interface {
	Config() *models.Config
	GetSystemQueries() graphql.Fields
	GetSystemMutations() graphql.Fields
	GetExtensionRouter() *echo.Group
	RegisterPluginID(id string)
	PluginIDs() []string
	TryExecutePlugin(ctx context.Context, pluginID, fnName, fnType string, args, contextData interface{}) (interface{}, error)
	LoadProjectSpecificPluginsForCache(ctx context.Context, cache *models.ApplicationCache) error
}

// PluginHost loads and supervises third-party plugins (HashiCorp, empty no-op, etc.).
type PluginHost interface {
	Load(ctx context.Context, runtime PluginRuntime) error
	Wait()
	StartMonitoring(ctx context.Context, runtime PluginRuntime)
	PrintRoutes()
}

package resolver

import (
	"context"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/tailor-platform/graphql"
)

var _ interfaces.PluginRuntime = (*GraphQLServer)(nil)

func (s *GraphQLServer) Config() *models.Config {
	return s.Cfg
}

// GetSystemQueries returns the current system query fields (read-only).
func (s *GraphQLServer) GetSystemQueries() graphql.Fields {
	queries, _ := s.SystemFieldsSnapshot()
	return queries
}

// GetSystemMutations returns the current system mutation fields (read-only).
func (s *GraphQLServer) GetSystemMutations() graphql.Fields {
	_, mutations := s.SystemFieldsSnapshot()
	return mutations
}

func (s *GraphQLServer) GetExtensionRouter() *echo.Group {
	return s.ExtensionRouter
}

func (s *GraphQLServer) RegisterPluginID(id string) {
	s.addInstalledHCPlugin(id)
}

func (s *GraphQLServer) PluginIDs() []string {
	return s.GetHashiCorpPluginIDs()
}

func (s *GraphQLServer) TryExecutePlugin(ctx context.Context, pluginID, fnName, fnType string, args, contextData interface{}) (interface{}, error) {
	argMap, _ := args.(map[string]interface{})
	return s.executePluginGraphQLResolver(ctx, pluginID, fnName, fnType, argMap)
}

func (s *GraphQLServer) LoadProjectSpecificPluginsForCache(ctx context.Context, cache *models.ApplicationCache) error {
	return s.LoadProjectSpecificPlugins(ctx, cache)
}

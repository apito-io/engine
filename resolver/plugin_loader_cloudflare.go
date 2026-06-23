//go:build cloudflare

package resolver

import (
	"context"
	"errors"

	"github.com/apito-io/engine/models"
)

// PluginMonitor is a no-op stub on Workers.
type PluginMonitor struct{}

func NewPluginMonitor(_ *GraphQLServer) *PluginMonitor { return &PluginMonitor{} }

func (pm *PluginMonitor) StartMonitoring(_ context.Context) {}
func (pm *PluginMonitor) StopMonitoring()                 {}

// LoadPlugins is a no-op when PluginHost is empty (Workers build).
func (s *GraphQLServer) LoadPlugins(_ context.Context) error { return nil }

func (s *GraphQLServer) LoadProjectSpecificPlugins(_ context.Context, _ *models.ApplicationCache) error {
	return nil
}

func (s *GraphQLServer) tryGetPluginNoBlock(_ string) *models.HashiCorpPluginCache {
	return nil
}

func (s *GraphQLServer) executePluginGraphQLResolver(_ context.Context, _, _, _ string, _ map[string]interface{}) (interface{}, error) {
	return nil, errors.New("plugins not available in Workers build")
}

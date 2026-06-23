//go:build !cloudflare

package plugin

import (
	"context"
	"fmt"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/types/protobuff"
	"time"
)

// PluginHealthCheck represents the health status of a plugin
type PluginHealthCheck struct {
	Status      string `json:"status"`
	Message     string `json:"message"`
	LastChecked int64  `json:"last_checked"`
}

// PluginMetrics represents metrics collected for a plugin
type PluginMetrics struct {
	RequestCount    int64   `json:"request_count"`
	ErrorCount      int64   `json:"error_count"`
	AverageLatency  float64 `json:"average_latency"`
	LastRequestTime int64   `json:"last_request_time"`
	MemoryUsage     int64   `json:"memory_usage"`
}

// HashiCorpPluginManager handles lifecycle operations for HashiCorp plugins
type HashiCorpPluginManager struct {
	cache map[string]*models.HashiCorpPluginCache
}

// NewHashiCorpPluginManager creates a new plugin manager
func NewHashiCorpPluginManager() *HashiCorpPluginManager {
	return &HashiCorpPluginManager{
		cache: make(map[string]*models.HashiCorpPluginCache),
	}
}

// GetPlugin retrieves a plugin from the cache
func (pm *HashiCorpPluginManager) GetPlugin(pluginID string) (*models.HashiCorpPluginCache, bool) {
	plugin, exists := pm.cache[pluginID]
	return plugin, exists
}

// RegisterPlugin adds a plugin to the cache
func (pm *HashiCorpPluginManager) RegisterPlugin(pluginID string, cache *models.HashiCorpPluginCache) {
	pm.cache[pluginID] = cache
}

// UnregisterPlugin removes a plugin from the cache and cleans up
func (pm *HashiCorpPluginManager) UnregisterPlugin(pluginID string) error {
	if cache, exists := pm.cache[pluginID]; exists {
		if cache.Client != nil {
			cache.Client.Kill()
		}
		delete(pm.cache, pluginID)
		return nil
	}
	return fmt.Errorf("plugin %s not found", pluginID)
}

// GetAllPlugins returns all registered plugins
func (pm *HashiCorpPluginManager) GetAllPlugins() map[string]*models.HashiCorpPluginCache {
	result := make(map[string]*models.HashiCorpPluginCache)
	for k, v := range pm.cache {
		result[k] = v
	}
	return result
}

// HealthCheckPlugin performs a health check on a specific plugin
func (pm *HashiCorpPluginManager) HealthCheckPlugin(ctx context.Context, pluginID string) (*PluginHealthCheck, error) {
	cache, exists := pm.GetPlugin(pluginID)
	if !exists {
		return &PluginHealthCheck{
			Status:      "NOT_FOUND",
			Message:     fmt.Sprintf("Plugin %s not found in cache", pluginID),
			LastChecked: time.Now().Unix(),
		}, nil
	}

	// Check if client is alive
	if cache.Client == nil {
		return &PluginHealthCheck{
			Status:      "CLIENT_NULL",
			Message:     "Plugin client is null",
			LastChecked: time.Now().Unix(),
		}, nil
	}

	// Check if plugin process has exited
	if cache.Client.Exited() {
		return &PluginHealthCheck{
			Status:      "EXITED",
			Message:     "Plugin process has exited",
			LastChecked: time.Now().Unix(),
		}, nil
	}

	return &PluginHealthCheck{
		Status:      "HEALTHY",
		Message:     "Plugin is running normally",
		LastChecked: time.Now().Unix(),
	}, nil
}

// HealthCheckAllPlugins performs health checks on all registered plugins
func (pm *HashiCorpPluginManager) HealthCheckAllPlugins(ctx context.Context) map[string]*PluginHealthCheck {
	results := make(map[string]*PluginHealthCheck)

	for pluginID := range pm.cache {
		health, _ := pm.HealthCheckPlugin(ctx, pluginID)
		results[pluginID] = health
	}

	return results
}

// RestartPlugin attempts to restart a failed plugin
func (pm *HashiCorpPluginManager) RestartPlugin(ctx context.Context, pluginID string, details *protobuff.PluginDetails) error {
	// First, unregister the existing plugin
	err := pm.UnregisterPlugin(pluginID)
	if err != nil {
		return fmt.Errorf("failed to unregister plugin %s: %w", pluginID, err)
	}

	// TODO: This would need access to the plugin loader
	// For now, return an error indicating manual restart is needed
	return fmt.Errorf("plugin restart not implemented - manual restart required for plugin %s", pluginID)
}

// GetPluginsByType returns all plugins of a specific type
func (pm *HashiCorpPluginManager) GetPluginsByType(pluginType protobuff.PluginType) []*models.HashiCorpPluginCache {
	var result []*models.HashiCorpPluginCache

	for _, cache := range pm.cache {
		if cache.PluginConfigurations != nil && cache.PluginConfigurations.Type == pluginType {
			result = append(result, cache)
		}
	}

	return result
}

// GetPluginMetrics collects metrics for a specific plugin
func (pm *HashiCorpPluginManager) GetPluginMetrics(pluginID string) (*PluginMetrics, error) {
	_, exists := pm.GetPlugin(pluginID)
	if !exists {
		return nil, fmt.Errorf("plugin %s not found", pluginID)
	}

	// For now, return basic metrics
	// In a real implementation, you'd collect these metrics over time
	return &PluginMetrics{
		RequestCount:    0, // Would be tracked over time
		ErrorCount:      0, // Would be tracked over time
		AverageLatency:  0, // Would be calculated over time
		LastRequestTime: 0, // Would be updated on each request
		MemoryUsage:     0, // Would require process monitoring
	}, nil
}

// ShutdownAllPlugins gracefully shuts down all plugins
func (pm *HashiCorpPluginManager) ShutdownAllPlugins() error {
	var errors []error

	for pluginID := range pm.cache {
		if err := pm.UnregisterPlugin(pluginID); err != nil {
			errors = append(errors, fmt.Errorf("failed to shutdown plugin %s: %w", pluginID, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors occurred during shutdown: %v", errors)
	}

	return nil
}

// ValidatePlugin checks if a plugin is properly configured and functional
func (pm *HashiCorpPluginManager) ValidatePlugin(ctx context.Context, pluginID string) error {
	cache, exists := pm.GetPlugin(pluginID)
	if !exists {
		return fmt.Errorf("plugin %s not found", pluginID)
	}

	if cache.Client == nil {
		return fmt.Errorf("plugin %s has no client", pluginID)
	}

	if cache.RPCClient == nil {
		return fmt.Errorf("plugin %s has no RPC client", pluginID)
	}

	if cache.PluginConfigurations == nil {
		return fmt.Errorf("plugin %s has no configuration", pluginID)
	}

	// Check if plugin process is still alive
	if cache.Client.Exited() {
		return fmt.Errorf("plugin %s process has exited", pluginID)
	}

	return nil
}

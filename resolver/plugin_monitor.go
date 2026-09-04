package resolver

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/types/protobuff"
)

// Plugin health monitoring constants
const (
	HEALTH_CHECK_INTERVAL      = 30 * time.Second // Check plugin health every 30 seconds
	RESTART_BACKOFF_BASE       = 1 * time.Second  // Base backoff time between restart attempts
	MAX_RESTART_ATTEMPTS       = 5                // Maximum restart attempts before marking as failed
	RESTART_BACKOFF_MULTIPLIER = 2                // Exponential backoff multiplier
	PLUGIN_STARTUP_TIMEOUT     = 10 * time.Second // Timeout for plugin startup
)

// PluginHealthStatus represents the health status of a plugin
type PluginHealthStatus struct {
	PluginID            string
	IsHealthy           bool
	LastHealthCheck     time.Time
	RestartAttempts     int
	LastRestartTime     time.Time
	ConsecutiveFailures int
	Status              string
}

// PluginMonitor handles health monitoring and restart logic for plugins
type PluginMonitor struct {
	server       *GraphQLServer
	healthStatus map[string]*PluginHealthStatus
	healthMutex  sync.RWMutex
	stopChannel  chan struct{}
	restartQueue chan string
	monitoring   bool
}

// NewPluginMonitor creates a new plugin monitor
func NewPluginMonitor(server *GraphQLServer) *PluginMonitor {
	return &PluginMonitor{
		server:       server,
		healthStatus: make(map[string]*PluginHealthStatus),
		stopChannel:  make(chan struct{}),
		restartQueue: make(chan string, 100), // Buffer for restart requests
		monitoring:   false,
	}
}

// StartMonitoring begins the plugin health monitoring process
func (pm *PluginMonitor) StartMonitoring(ctx context.Context) {
	if pm.monitoring {
		return
	}

	pm.monitoring = true

	// Start health check routine
	go pm.healthCheckRoutine(ctx)

	// Start restart handler routine
	go pm.restartHandlerRoutine(ctx)

	fmt.Println("🔍 [PLUGIN-MONITOR] Plugin health monitoring started")
}

// StopMonitoring stops the plugin health monitoring
func (pm *PluginMonitor) StopMonitoring() {
	if !pm.monitoring {
		return
	}

	pm.monitoring = false
	close(pm.stopChannel)
	fmt.Println("🔍 [PLUGIN-MONITOR] Plugin health monitoring stopped")
}

// healthCheckRoutine periodically checks plugin health
func (pm *PluginMonitor) healthCheckRoutine(ctx context.Context) {
	ticker := time.NewTicker(HEALTH_CHECK_INTERVAL)
	defer ticker.Stop()

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Plugin health check routine panic: %v\n", r)
		}
		fmt.Println("🔍 [PLUGIN-MONITOR] Health check routine stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("🔍 [PLUGIN-MONITOR] Health check routine context cancelled")
			return
		case <-pm.stopChannel:
			fmt.Println("🔍 [PLUGIN-MONITOR] Health check routine stop signal received")
			return
		case <-ticker.C:
			// Add timeout to health checks to prevent blocking
			healthCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			pm.performHealthChecks(healthCtx)
			cancel()
		}
	}
}

// restartHandlerRoutine handles plugin restart requests
func (pm *PluginMonitor) restartHandlerRoutine(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Plugin restart handler routine panic: %v\n", r)
		}
		fmt.Println("🔍 [PLUGIN-MONITOR] Restart handler routine stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("🔍 [PLUGIN-MONITOR] Restart handler routine context cancelled")
			return
		case <-pm.stopChannel:
			fmt.Println("🔍 [PLUGIN-MONITOR] Restart handler routine stop signal received")
			return
		case pluginID := <-pm.restartQueue:
			// Add timeout to restart operations to prevent blocking
			restartCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
			pm.handlePluginRestart(restartCtx, pluginID)
			cancel()
		}
	}
}

// performHealthChecks checks the health of all monitored plugins
func (pm *PluginMonitor) performHealthChecks(ctx context.Context) {
	pm.healthMutex.RLock()
	pluginIDs := make([]string, 0, len(pm.healthStatus))
	for pluginID := range pm.healthStatus {
		pluginIDs = append(pluginIDs, pluginID)
	}
	pm.healthMutex.RUnlock()

	for _, pluginID := range pluginIDs {
		pm.checkPluginHealth(ctx, pluginID)
	}
}

// checkPluginHealth checks the health of a specific plugin
func (pm *PluginMonitor) checkPluginHealth(ctx context.Context, pluginID string) {
	pm.healthMutex.Lock()
	_, exists := pm.healthStatus[pluginID]
	if !exists {
		pm.healthMutex.Unlock()
		return
	}
	pm.healthMutex.Unlock()

	// Try to get the plugin from cache
	plugin := pm.server.tryGetPluginNoBlock(pluginID)
	if plugin == nil {
		pm.markPluginUnhealthy(pluginID, "Plugin not found in cache")
		pm.scheduleRestart(pluginID)
		return
	}

	// Check if the client is still alive
	if plugin.Client.Exited() {
		pm.markPluginUnhealthy(pluginID, "Plugin process exited")
		pm.scheduleRestart(pluginID)
		return
	}

	// Try to get RPC client and perform ping
	rpcClient, err := plugin.Client.Client()
	if err != nil {
		pm.markPluginUnhealthy(pluginID, fmt.Sprintf("Failed to get RPC client: %v", err))
		pm.scheduleRestart(pluginID)
		return
	}

	// Dispense the plugin and try to ping it
	raw, err := rpcClient.Dispense(plugin.PluginConfigurations.ExportedVariable)
	if err != nil {
		pm.markPluginUnhealthy(pluginID, fmt.Sprintf("Failed to dispense plugin: %v", err))
		pm.scheduleRestart(pluginID)
		return
	}

	loadedPlugin, ok := raw.(*pluginService.HashiCorpNormalPluginGRPC)
	if !ok {
		pm.markPluginUnhealthy(pluginID, "Plugin is not a valid universal plugin")
		pm.scheduleRestart(pluginID)
		return
	}

	// Try to ping the plugin (simple health check)
	ctxWithTimeout, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err = loadedPlugin.Execute(ctxWithTimeout, "health_check", "system", map[string]interface{}{}, map[string]interface{}{})
	if err != nil {
		// If health check fails, it might be a panic or network issue
		pm.markPluginUnhealthy(pluginID, fmt.Sprintf("Health check failed: %v", err))
		pm.scheduleRestart(pluginID)
		return
	}

	// Plugin is healthy
	pm.markPluginHealthy(pluginID)
}

// markPluginHealthy marks a plugin as healthy
func (pm *PluginMonitor) markPluginHealthy(pluginID string) {
	pm.healthMutex.Lock()
	defer pm.healthMutex.Unlock()

	if status, exists := pm.healthStatus[pluginID]; exists {
		status.IsHealthy = true
		status.LastHealthCheck = time.Now()
		status.ConsecutiveFailures = 0
		status.Status = "Healthy"

		fmt.Printf("✅ [PLUGIN-MONITOR] Plugin health check passed: %s\n", pluginID)
		pm.server.EmitPluginStatusChanged(context.Background(), pluginID, "healthy", "")
	}
}

// markPluginUnhealthy marks a plugin as unhealthy
func (pm *PluginMonitor) markPluginUnhealthy(pluginID, reason string) {
	pm.healthMutex.Lock()
	defer pm.healthMutex.Unlock()

	if status, exists := pm.healthStatus[pluginID]; exists {
		status.IsHealthy = false
		status.LastHealthCheck = time.Now()
		status.ConsecutiveFailures++
		status.Status = reason

		fmt.Printf("❌ [PLUGIN-MONITOR] Plugin health check failed - ID: %s, Reason: %s, Consecutive Failures: %d\n",
			pluginID, reason, status.ConsecutiveFailures)
		pm.server.EmitPluginStatusChanged(context.Background(), pluginID, "unhealthy", reason)
	}
}

// scheduleRestart schedules a plugin for restart
func (pm *PluginMonitor) scheduleRestart(pluginID string) {
	select {
	case pm.restartQueue <- pluginID:
		fmt.Printf("📅 [PLUGIN-MONITOR] Plugin restart scheduled: %s\n", pluginID)
	default:
		fmt.Printf("⚠️ [PLUGIN-MONITOR] Restart queue full, skipping restart for plugin: %s\n", pluginID)
	}
}

// handlePluginRestart handles the restart of a specific plugin
func (pm *PluginMonitor) handlePluginRestart(ctx context.Context, pluginID string) {
	pm.healthMutex.Lock()
	status, exists := pm.healthStatus[pluginID]
	if !exists {
		pm.healthMutex.Unlock()
		return
	}

	// Check if we've exceeded max restart attempts
	if status.RestartAttempts >= MAX_RESTART_ATTEMPTS {
		fmt.Printf("🚫 [PLUGIN-MONITOR] Plugin exceeded maximum restart attempts, marking as permanently failed - ID: %s, Attempts: %d\n",
			pluginID, status.RestartAttempts)
		status.Status = "Permanently Failed"
		pm.healthMutex.Unlock()
		pm.server.EmitPluginStatusChanged(context.Background(), pluginID, "permanently_failed", "max restart attempts exceeded")
		return
	}

	status.RestartAttempts++
	status.LastRestartTime = time.Now()
	pm.healthMutex.Unlock()

	// Calculate backoff time
	backoffTime := time.Duration(status.RestartAttempts) * RESTART_BACKOFF_BASE *
		time.Duration(1<<uint(status.RestartAttempts-1)) // Exponential backoff

	fmt.Printf("🔄 [PLUGIN-MONITOR] Restarting plugin after backoff - ID: %s, Attempt: %d, Backoff: %.2f seconds\n",
		pluginID, status.RestartAttempts, backoffTime.Seconds())

	// Wait for backoff
	select {
	case <-ctx.Done():
		return
	case <-time.After(backoffTime):
	}

	// Attempt to restart the plugin
	if err := pm.restartPlugin(ctx, pluginID); err != nil {
		fmt.Printf("❌ [PLUGIN-MONITOR] Failed to restart plugin - ID: %s, Attempt: %d, Error: %v\n",
			pluginID, status.RestartAttempts, err)

		// Schedule another restart attempt
		pm.scheduleRestart(pluginID)
	} else {
		fmt.Printf("✅ [PLUGIN-MONITOR] Plugin restarted successfully - ID: %s, Attempt: %d\n",
			pluginID, status.RestartAttempts)

		// Reset restart attempts on successful restart
		pm.healthMutex.Lock()
		if status, exists := pm.healthStatus[pluginID]; exists {
			status.RestartAttempts = 0
			status.IsHealthy = true
			status.Status = "Restarted"
		}
		pm.healthMutex.Unlock()
		pm.server.EmitPluginStatusChanged(context.Background(), pluginID, "restarted", "")
	}
}

// restartPlugin performs the actual plugin restart
func (pm *PluginMonitor) restartPlugin(ctx context.Context, pluginID string) error {
	plugin := pm.server.tryGetPluginNoBlock(pluginID)

	var pluginConfig *protobuff.PluginDetails
	if plugin != nil {
		pluginConfig = plugin.PluginConfigurations
		// Kill the existing plugin process
		fmt.Printf("🔪 [PLUGIN-MONITOR] Killing existing plugin process: %s\n", pluginID)
		plugin.Client.Kill()
		// Remove from cache
		pm.server.removeHashiCorpPlugin(pluginID)
	} else {
		// Plugin not in cache -- load config from YAML registry
		registry, err := pluginService.LoadHashiCorpPluginRegistry(pm.server.Cfg)
		if err != nil {
			return fmt.Errorf("plugin %s not in cache and registry load failed: %w", pluginID, err)
		}
		details, exists := registry[pluginID]
		if !exists {
			return fmt.Errorf("plugin %s not found in cache or registry", pluginID)
		}
		if !details.Enable {
			return fmt.Errorf("plugin %s is disabled in registry", pluginID)
		}
		pluginConfig = details
	}

	// Wait a bit for cleanup with context timeout check
	select {
	case <-time.After(1 * time.Second):
	case <-ctx.Done():
		return fmt.Errorf("restart cancelled due to context timeout")
	}

	// Restart the plugin
	dir := filepath.Join(pm.server.Cfg.PluginPath, pluginID)

	// Load the plugin using existing method
	_, err := pm.server.LoadHashiCorpPlugin(ctx, dir, pluginConfig)
	if err != nil {
		return fmt.Errorf("failed to load plugin %s: %w", pluginID, err)
	}

	fmt.Printf("🚀 [PLUGIN-MONITOR] Plugin restarted successfully: %s\n", pluginID)

	return nil
}

// RegisterPlugin registers a plugin for health monitoring
func (pm *PluginMonitor) RegisterPlugin(pluginID string) {
	pm.healthMutex.Lock()
	defer pm.healthMutex.Unlock()

	pm.healthStatus[pluginID] = &PluginHealthStatus{
		PluginID:            pluginID,
		IsHealthy:           true,
		LastHealthCheck:     time.Now(),
		RestartAttempts:     0,
		ConsecutiveFailures: 0,
		Status:              "Healthy",
	}

	fmt.Printf("📝 [PLUGIN-MONITOR] Plugin registered for health monitoring: %s\n", pluginID)
}

// UnregisterPlugin removes a plugin from health monitoring
func (pm *PluginMonitor) UnregisterPlugin(pluginID string) {
	pm.healthMutex.Lock()
	defer pm.healthMutex.Unlock()

	delete(pm.healthStatus, pluginID)
	fmt.Printf("📝 [PLUGIN-MONITOR] Plugin unregistered from health monitoring: %s\n", pluginID)
}

// GetPluginHealthStatus returns the health status of all monitored plugins
func (pm *PluginMonitor) GetPluginHealthStatus() map[string]*PluginHealthStatus {
	pm.healthMutex.RLock()
	defer pm.healthMutex.RUnlock()

	status := make(map[string]*PluginHealthStatus)
	for pluginID, pluginStatus := range pm.healthStatus {
		status[pluginID] = &PluginHealthStatus{
			PluginID:            pluginStatus.PluginID,
			IsHealthy:           pluginStatus.IsHealthy,
			LastHealthCheck:     pluginStatus.LastHealthCheck,
			RestartAttempts:     pluginStatus.RestartAttempts,
			LastRestartTime:     pluginStatus.LastRestartTime,
			ConsecutiveFailures: pluginStatus.ConsecutiveFailures,
			Status:              pluginStatus.Status,
		}
	}

	return status
}

// RestartPlugin provides a public interface to restart a specific plugin
// This method is intended for use by controllers and other external components
func (pm *PluginMonitor) RestartPlugin(ctx context.Context, pluginID string) error {
	return pm.restartPlugin(ctx, pluginID)
}

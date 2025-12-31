package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/apito-io/engine/database/project"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

// ConnectionConfig holds the database connection configuration for a tenant
type ConnectionConfig struct {
	TenantID    string
	Driver      string
	ConnString  string
	MaxIdleConn int
	MaxOpenConn int
	MaxLifetime time.Duration
}

// Closeable is an interface for connections that can be closed
type Closeable interface {
	Close() error
}

// Connection represents a database connection with metadata
type Connection struct {
	TenantID     string
	DBConn       interfaces.ProjectDBInterface
	LastAccessed time.Time
	IsActive     bool
}

// ConnectionManager manages database connections for multiple tenants
type ConnectionManager struct {
	cfg *models.Config
	// Cache for active connections
	activeConns *cache.Cache
	// Mutex for thread-safe operations
	mu sync.RWMutex
	// Maximum number of concurrent connections
	maxConnections int
	// Connection configs by tenant
	configs map[string]*models.DriverCredentials
	// Prevent thundering herd
	requestGroup singleflight.Group
	// For monitoring active connections
	stats ConnectionStats
	// Settings that is project dependent
	projectDependedSettings ProjectDependedSettings
	// System DB from here will be used in plugin functions
	systemDB interfaces.ApitoSystemDB
}

type ConnectionStats struct {
	ActiveConnections int
	CacheHits         int64
	CacheMisses       int64
	Evictions         int64
	CloseErrors       int64
}

type ProjectDependedSettings struct {
	ActivatedLogicExecutionProvidersID []string
	DefaultStorageProviderId           string
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(cfg *models.Config, maxConns int, systemDB interfaces.ApitoSystemDB) *ConnectionManager {
	cm := &ConnectionManager{
		cfg:                     cfg,
		maxConnections:          maxConns,
		configs:                 make(map[string]*models.DriverCredentials),
		stats:                   ConnectionStats{},
		projectDependedSettings: ProjectDependedSettings{},
		systemDB:                systemDB,
	}

	// Create cache with OnEvicted callback to properly track and close connections
	cm.activeConns = cache.New(2*time.Hour, 30*time.Minute)
	cm.activeConns.OnEvicted(func(tenantID string, item interface{}) {
		cm.handleEviction(tenantID, item)
	})

	return cm
}

// handleEviction is called when a connection is evicted from cache (expired or deleted)
func (cm *ConnectionManager) handleEviction(tenantID string, item interface{}) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.stats.Evictions++

	// Decrement the active connections counter
	if cm.stats.ActiveConnections > 0 {
		cm.stats.ActiveConnections--
	}

	// Properly close the database connection if it implements Closeable
	if conn, ok := item.(*Connection); ok && conn != nil && conn.DBConn != nil {
		if closeable, ok := conn.DBConn.(Closeable); ok {
			if err := closeable.Close(); err != nil {
				cm.stats.CloseErrors++
				log.Printf("Error closing connection for tenant %s: %v", tenantID, err)
			} else {
				log.Printf("Successfully closed evicted connection for tenant: %s", tenantID)
			}
		}
	}
}

// AddDriverCredentials adds a new connection configuration for a tenant
func (cm *ConnectionManager) AddDriverCredentials(ctx context.Context, config *models.DriverCredentials) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var projectID string
	if ctx.Value("project_id") != nil {
		projectID = ctx.Value("project_id").(string)
	} else {
		projectID = config.ProjectID
	}
	cm.configs[projectID] = config
}

// GetConnection returns a database connection for the given tenant
func (cm *ConnectionManager) GetConnection(ctx context.Context, tenantID string) (*Connection, error) {
	// Try to get from cache first
	if cached, found := cm.activeConns.Get(tenantID); found {
		conn := cached.(*Connection)
		// Update last accessed time for proper LRU behavior
		conn.LastAccessed = time.Now()
		cm.mu.Lock()
		cm.stats.CacheHits++
		cm.mu.Unlock()
		return conn, nil
	}

	// Use singleflight to prevent multiple simultaneous connections for the same tenant
	result, err, _ := cm.requestGroup.Do(tenantID, func() (interface{}, error) {
		return cm.createConnection(ctx, tenantID)
	})
	if err != nil {
		return nil, err
	}

	return result.(*Connection), nil
}

func (cm *ConnectionManager) createConnection(ctx context.Context, tenantID string) (*Connection, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check: connection might have been created by another goroutine while we were waiting
	if cached, found := cm.activeConns.Get(tenantID); found {
		conn := cached.(*Connection)
		conn.LastAccessed = time.Now()
		cm.stats.CacheHits++
		return conn, nil
	}

	// Check if we've reached max connections
	if cm.stats.ActiveConnections >= cm.maxConnections {
		// Try to clean up unused connections
		cm.cleanupLocked()

		if cm.stats.ActiveConnections >= cm.maxConnections {
			log.Printf("Connection limit reached: active=%d, max=%d, cache_items=%d",
				cm.stats.ActiveConnections, cm.maxConnections, cm.activeConns.ItemCount())
			return nil, errors.New("maximum connection limit reached")
		}
	}

	config, exists := cm.configs[tenantID]
	if !exists {
		return nil, fmt.Errorf("tenant configuration not found for: %s", tenantID)
	}

	projectDriver, err := project.GetProjectDriver(config, cm.cfg)
	if err != nil {
		log.Printf("Project db connection create error for tenant %s: %v", tenantID, err)
		return nil, err
	}

	conn := &Connection{
		TenantID:     tenantID,
		DBConn:       projectDriver,
		LastAccessed: time.Now(),
		IsActive:     true,
	}

	// Store in cache with metadata
	cm.activeConns.Set(tenantID, conn, cache.DefaultExpiration)

	cm.stats.ActiveConnections++
	cm.stats.CacheMisses++

	log.Printf("Created new connection for tenant: %s (active: %d)", tenantID, cm.stats.ActiveConnections)

	return conn, nil
}

// cleanupLocked removes least recently used connections when approaching capacity
// NOTE: This must be called while holding cm.mu lock
// NOTE: We manually handle cleanup here to avoid deadlock with OnEvicted callback
func (cm *ConnectionManager) cleanupLocked() {
	items := cm.activeConns.Items()

	if len(items) == 0 {
		// Cache is empty but counter is high - this indicates counter desync
		// Reset counter to match actual cache state
		if cm.stats.ActiveConnections > 0 {
			log.Printf("Warning: Counter desync detected. Counter=%d, CacheItems=0. Resetting counter.",
				cm.stats.ActiveConnections)
			cm.stats.ActiveConnections = 0
		}
		return
	}

	// Sort connections by last accessed time
	type connectionAge struct {
		tenantID     string
		lastAccessed time.Time
		conn         *Connection
	}

	var connections []connectionAge
	for tenantID, item := range items {
		// Fix: type assertion should be *Connection (pointer) not Connection
		if conn, ok := item.Object.(*Connection); ok {
			connections = append(connections, connectionAge{
				tenantID:     tenantID,
				lastAccessed: conn.LastAccessed,
				conn:         conn,
			})
		}
	}

	// Sort by last accessed time (oldest first)
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].lastAccessed.Before(connections[j].lastAccessed)
	})

	// Remove oldest 20% of connections (minimum 1 if we have any)
	numToRemove := len(connections) / 5
	if numToRemove == 0 && len(connections) > 0 {
		numToRemove = 1
	}

	log.Printf("Cleanup: removing %d oldest connections out of %d", numToRemove, len(connections))

	// Collect tenant IDs to remove
	toRemove := make([]connectionAge, numToRemove)
	for i := 0; i < numToRemove; i++ {
		toRemove[i] = connections[i]
	}

	// Release lock before calling Delete to avoid deadlock with OnEvicted
	cm.mu.Unlock()

	// Delete items - this triggers OnEvicted callback which handles counter and close
	for _, item := range toRemove {
		cm.activeConns.Delete(item.tenantID)
	}

	// Re-acquire lock for the caller
	cm.mu.Lock()
}

// GetStats returns current connection statistics
func (cm *ConnectionManager) GetStats() ConnectionStats {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	// Return a copy with actual cache item count for debugging
	stats := cm.stats
	return stats
}

// GetDetailedStats returns detailed statistics including cache state
func (cm *ConnectionManager) GetDetailedStats() map[string]interface{} {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return map[string]interface{}{
		"active_connections": cm.stats.ActiveConnections,
		"cache_items":        cm.activeConns.ItemCount(),
		"cache_hits":         cm.stats.CacheHits,
		"cache_misses":       cm.stats.CacheMisses,
		"evictions":          cm.stats.Evictions,
		"close_errors":       cm.stats.CloseErrors,
		"max_connections":    cm.maxConnections,
	}
}

// CloseAll closes all active connections - call this on graceful shutdown
func (cm *ConnectionManager) CloseAll() {
	// Get list of tenant IDs to close
	items := cm.activeConns.Items()
	tenantIDs := make([]string, 0, len(items))
	for tenantID := range items {
		tenantIDs = append(tenantIDs, tenantID)
	}

	log.Printf("Closing all %d connections...", len(tenantIDs))

	// Delete each - this triggers OnEvicted which handles locking internally
	for _, tenantID := range tenantIDs {
		cm.activeConns.Delete(tenantID)
	}

	cm.mu.RLock()
	activeCount := cm.stats.ActiveConnections
	cm.mu.RUnlock()

	log.Printf("All connections closed. Final stats: active=%d", activeCount)
}

// RemoveConnection explicitly removes a connection for a tenant
func (cm *ConnectionManager) RemoveConnection(tenantID string) {
	cm.activeConns.Delete(tenantID) // This triggers OnEvicted
}

// SetProjectDefaultMediaPlugin sets project dependent settings
func (cm *ConnectionManager) SetProjectDefaultMediaPlugin(defaultStorageProviderId string) {
	cm.projectDependedSettings.DefaultStorageProviderId = defaultStorageProviderId
}

// SetProjectLogicExecutionProvider sets project dependent settings
func (cm *ConnectionManager) SetProjectLogicExecutionProvider(activatedLogicExecutionProvidersID string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check for duplicate
	for _, id := range cm.projectDependedSettings.ActivatedLogicExecutionProvidersID {
		if id == activatedLogicExecutionProvidersID {
			return
		}
	}

	// Append if not found
	cm.projectDependedSettings.ActivatedLogicExecutionProvidersID = append(cm.projectDependedSettings.ActivatedLogicExecutionProvidersID, activatedLogicExecutionProvidersID)
}

// GetProjectDependedSettings returns project dependent settings
func (cm *ConnectionManager) GetProjectDependedSettings() ProjectDependedSettings {
	return cm.projectDependedSettings
}

// IsHealthy returns true if the connection pool is under 90% capacity
func (cm *ConnectionManager) IsHealthy() bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.stats.ActiveConnections < cm.maxConnections*90/100
}

// GetUsagePercent returns the current connection pool usage percentage
func (cm *ConnectionManager) GetUsagePercent() float64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return float64(cm.stats.ActiveConnections) / float64(cm.maxConnections) * 100
}

// GetMaxConnections returns the maximum number of connections allowed
func (cm *ConnectionManager) GetMaxConnections() int {
	return cm.maxConnections
}

package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/apito-io/engine/database/project"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/telemetry"
	"github.com/patrickmn/go-cache"
	"golang.org/x/sync/singleflight"
)

// ConnectionConfig holds the database connection configuration for a scoped connection
type ConnectionConfig struct {
	ScopeKey    string
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
	ScopeKey     string
	DBConn       interfaces.ProjectDBInterface
	LastAccessed time.Time
	IsActive     bool
}

// ConnectionManager manages database connections for multiple scopes
type ConnectionManager struct {
	cfg *models.Config
	// Cache for active connections
	activeConns *cache.Cache
	// Mutex for thread-safe operations
	mu sync.RWMutex
	// Maximum number of concurrent connections
	maxConnections int
	// Connection configs by scope key
	configs map[string]*models.DriverCredentials
	// proDriverExtras holds pro-only credentials keyed like configs (e.g. *ProDriverCredentials for Firestore/DynamoDB).
	proDriverExtras map[string]interface{}
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
	cm.activeConns.OnEvicted(func(connKey string, item interface{}) {
		cm.handleEviction(connKey, item)
	})

	return cm
}

// handleEviction is called when a connection is evicted from cache (expired or deleted)
func (cm *ConnectionManager) handleEviction(connKey string, item interface{}) {
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
				log.Printf("Error closing connection for scope %s: %v", connKey, err)
			} else {
				log.Printf("Successfully closed evicted connection for scope: %s", connKey)
			}
		}
	}
}

// AddDriverCredentials adds a new connection configuration for a project
func (cm *ConnectionManager) AddDriverCredentials(ctx context.Context, config *models.DriverCredentials) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var projectID string
	if ctx.Value("project_id") != nil {
		projectID = ctx.Value("project_id").(string)
	} else {
		projectID = config.ProjectID
	}
	// Do NOT overwrite a complete registration with an incomplete one.
	// Project create / SetProjectDriverCredential stores full host+db; a later Init with
	// persisted apitoDB-only credentials must not clobber that.
	if existing, ok := cm.configs[projectID]; ok && existing != nil &&
		strings.TrimSpace(existing.Host) != "" && strings.TrimSpace(config.Host) == "" {
		return
	}
	cm.configs[projectID] = config
}

// AddProDriverExtras registers pro-only driver metadata for a project (or scope composite key). Nil removes the entry.
func (cm *ConnectionManager) AddProDriverExtras(projectID string, pro interface{}) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.proDriverExtras == nil {
		cm.proDriverExtras = make(map[string]interface{})
	}
	if pro == nil {
		delete(cm.proDriverExtras, projectID)
		return
	}
	cm.proDriverExtras[projectID] = pro
}

// AddScopedDriverCredentials registers explicit credentials for a composite project:scope cache key
// (e.g. after a hosted driver API returns a dedicated URL). Normally DeriveScopedCredentials + EnsureScopedCredentials is enough.
func (cm *ConnectionManager) AddScopedDriverCredentials(projectID, scopeKey string, config *models.DriverCredentials) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	key := ScopedConnectionCacheKey(projectID, scopeKey)
	if config != nil {
		c := *config
		c.ProjectID = key
		cm.configs[key] = &c
	}
}

// EnsureScopedCredentials registers derived per-scope credentials from the base project config if missing.
func (cm *ConnectionManager) EnsureScopedCredentials(projectID, scopeKey string) error {
	if scopeKey == "" {
		return errors.New("scope key is required")
	}
	key := ScopedConnectionCacheKey(projectID, scopeKey)
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if _, ok := cm.configs[key]; ok {
		return nil
	}
	base, ok := cm.configs[projectID]
	if !ok {
		return fmt.Errorf("base project configuration not found for: %s", projectID)
	}
	derived := DeriveScopedCredentials(base, projectID, scopeKey)
	if derived == nil {
		return fmt.Errorf("could not derive scoped credentials for project %s", projectID)
	}
	cm.configs[key] = derived
	return nil
}

// GetScopedConnection returns a pooled driver for projectID + scopeKey (composite cache key).
func (cm *ConnectionManager) GetScopedConnection(ctx context.Context, projectID, scopeKey string) (*Connection, error) {
	if err := cm.EnsureScopedCredentials(projectID, scopeKey); err != nil {
		return nil, err
	}
	key := ScopedConnectionCacheKey(projectID, scopeKey)
	return cm.GetConnection(ctx, key)
}

// GetConnection returns a database connection for the given connection key
func (cm *ConnectionManager) GetConnection(ctx context.Context, connKey string) (_ *Connection, err error) {
	start := time.Now()
	projectID := connKey
	if i := strings.Index(connKey, ":"); i >= 0 {
		projectID = connKey[:i]
	}
	defer func() {
		if !telemetry.MetricsEnabled(cm.cfg) {
			return
		}
		eng := ""
		cm.mu.RLock()
		if cred := cm.configs[connKey]; cred != nil {
			eng = cred.Engine
		}
		cm.mu.RUnlock()
		telemetry.RecordPoolAcquire(ctx, cm.cfg, projectID, eng, err, time.Since(start))
	}()

	// Try to get from cache first
	if cached, found := cm.activeConns.Get(connKey); found {
		conn := cached.(*Connection)
		// Returning cached drivers directly avoids closing a shared *sql.DB while
		// concurrent GraphQL field resolvers are still queued on the same handle.
		conn.LastAccessed = time.Now()
		cm.mu.Lock()
		cm.stats.CacheHits++
		cm.mu.Unlock()
		return conn, nil
	}

	// Use singleflight to prevent multiple simultaneous connections for the same key
	result, err, _ := cm.requestGroup.Do(connKey, func() (interface{}, error) {
		return cm.createConnection(ctx, connKey)
	})
	if err != nil {
		return nil, err
	}

	return result.(*Connection), nil
}

func (cm *ConnectionManager) createConnection(ctx context.Context, connKey string) (*Connection, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Double-check: connection might have been created by another goroutine while we were waiting
	if cached, found := cm.activeConns.Get(connKey); found {
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

	config, exists := cm.configs[connKey]
	if !exists {
		return nil, fmt.Errorf("connection configuration not found for: %s", connKey)
	}

	proExtras := cm.proDriverExtras[connKey]
	projectDriver, err := project.GetProjectDriverWithConfig(cm.cfg, config, proExtras)
	if err != nil {
		log.Printf("Project db connection create error for %s: %v", connKey, err)
		return nil, err
	}

	conn := &Connection{
		ScopeKey:     connKey,
		DBConn:       projectDriver,
		LastAccessed: time.Now(),
		IsActive:     true,
	}

	// Store in cache with metadata
	cm.activeConns.Set(connKey, conn, cache.DefaultExpiration)

	cm.stats.ActiveConnections++
	cm.stats.CacheMisses++

	log.Printf("Created new connection for: %s (active: %d)", connKey, cm.stats.ActiveConnections)

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
		connKey      string
		lastAccessed time.Time
		conn         *Connection
	}

	var connections []connectionAge
	for key, item := range items {
		if conn, ok := item.Object.(*Connection); ok {
			connections = append(connections, connectionAge{
				connKey:      key,
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

	// Collect keys to remove
	toRemove := make([]connectionAge, numToRemove)
	for i := 0; i < numToRemove; i++ {
		toRemove[i] = connections[i]
	}

	// Release lock before calling Delete to avoid deadlock with OnEvicted
	cm.mu.Unlock()

	// Delete items - this triggers OnEvicted callback which handles counter and close
	for _, item := range toRemove {
		cm.activeConns.Delete(item.connKey)
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
	items := cm.activeConns.Items()
	keys := make([]string, 0, len(items))
	for k := range items {
		keys = append(keys, k)
	}

	log.Printf("Closing all %d connections...", len(keys))

	// Delete each - this triggers OnEvicted which handles locking internally
	for _, k := range keys {
		cm.activeConns.Delete(k)
	}

	cm.mu.RLock()
	activeCount := cm.stats.ActiveConnections
	cm.mu.RUnlock()

	log.Printf("All connections closed. Final stats: active=%d", activeCount)
}

// RemoveConnection explicitly removes a connection by key
func (cm *ConnectionManager) RemoveConnection(connKey string) {
	cm.activeConns.Delete(connKey) // This triggers OnEvicted
}

// GetConfig returns the configuration used by this connection manager.
func (cm *ConnectionManager) GetConfig() *models.Config {
	return cm.cfg
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

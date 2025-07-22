package database

import (
	"context"
	"errors"
	"fmt"
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

// Connection represents a database connection with metadata
type Connection struct {
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
}

type ProjectDependedSettings struct {
	ActivatedLogicExecutionProvidersID []string
	DefaultStorageProviderId           string
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(cfg *models.Config, maxConns int, systemDB interfaces.ApitoSystemDB) *ConnectionManager {
	return &ConnectionManager{
		cfg:                     cfg,
		activeConns:             cache.New(30*time.Minute, 10*time.Minute),
		maxConnections:          maxConns,
		configs:                 make(map[string]*models.DriverCredentials),
		stats:                   ConnectionStats{},
		projectDependedSettings: ProjectDependedSettings{},
		systemDB:                systemDB,
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
	if conn, found := cm.activeConns.Get(tenantID); found {
		cm.stats.CacheHits++
		return conn.(*Connection), nil
	}

	// Use singleflight to prevent multiple simultaneous connections for the same tenant
	conn, err, _ := cm.requestGroup.Do(tenantID, func() (interface{}, error) {
		return cm.createConnection(ctx, tenantID)
	})
	if err != nil {
		return nil, err
	}

	return conn.(*Connection), nil
}

func (cm *ConnectionManager) createConnection(ctx context.Context, tenantID string) (*Connection, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if we've reached max connections
	if cm.stats.ActiveConnections >= cm.maxConnections {
		// Try to clean up unused connections
		cm.cleanup()

		if cm.stats.ActiveConnections >= cm.maxConnections {
			return nil, errors.New("maximum connection limit reached")
		}
	}

	config, exists := cm.configs[tenantID]
	if !exists {
		return nil, errors.New("tenant configuration not found")
	}

	projectDriver, err := project.GetProjectDriver(config, cm.cfg)
	if err != nil {
		fmt.Println("project db connection create error:", err.Error())
		return nil, err
	}

	conn := &Connection{
		DBConn:       projectDriver,
		LastAccessed: time.Now(),
		IsActive:     true,
	}

	// Store in cache with metadata
	cm.activeConns.Set(tenantID, conn, cache.DefaultExpiration)

	cm.stats.ActiveConnections++
	cm.stats.CacheMisses++

	return conn, nil
}

// cleanup removes least recently used connections when approaching capacity
func (cm *ConnectionManager) cleanup() {
	items := cm.activeConns.Items()

	// Sort connections by last accessed time
	type connectionAge struct {
		tenantID     string
		lastAccessed time.Time
	}

	var connections []connectionAge
	for tenantID, item := range items {
		conn := item.Object.(Connection)
		connections = append(connections, connectionAge{
			tenantID:     tenantID,
			lastAccessed: conn.LastAccessed,
		})
	}

	// Sort by last accessed time
	sort.Slice(connections, func(i, j int) bool {
		return connections[i].lastAccessed.Before(connections[j].lastAccessed)
	})

	// Remove oldest 20% of connections
	numToRemove := len(connections) / 5
	for i := 0; i < numToRemove; i++ {
		cm.activeConns.Delete(connections[i].tenantID)
		cm.stats.ActiveConnections--
	}
}

// GetStats returns current connection statistics
func (cm *ConnectionManager) GetStats() ConnectionStats {
	return cm.stats
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

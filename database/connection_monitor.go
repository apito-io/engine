package database

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Connection monitoring constants
const (
	connectionMonitorInterval = 60 * time.Second // Check connection health every 60 seconds
	warningThresholdPercent   = 70               // Warn when usage exceeds 70%
	criticalThresholdPercent  = 90               // Critical when usage exceeds 90%
	statsLogInterval          = 5 * time.Minute  // Log detailed stats every 5 minutes
)

// HealthLevel represents the health level of the connection pool
type HealthLevel string

const (
	HealthLevelHealthy  HealthLevel = "HEALTHY"
	HealthLevelWarning  HealthLevel = "WARNING"
	HealthLevelCritical HealthLevel = "CRITICAL"
)

// ConnectionHealthStatus represents the health status of the connection pool
type ConnectionHealthStatus struct {
	HealthLevel       HealthLevel
	ActiveConnections int
	MaxConnections    int
	CacheItems        int
	UsagePercent      float64
	CacheHits         int64
	CacheMisses       int64
	Evictions         int64
	CloseErrors       int64
	LastCheck         time.Time
	Message           string
}

// ConnectionMonitor handles health monitoring for the connection manager
type ConnectionMonitor struct {
	connectionManager *ConnectionManager
	healthStatus      ConnectionHealthStatus
	healthMutex       sync.RWMutex
	stopChannel       chan struct{}
	monitoring        bool
	startTime         time.Time
	lastStatsLog      time.Time
}

// NewConnectionMonitor creates a new connection monitor
func NewConnectionMonitor(cm *ConnectionManager) *ConnectionMonitor {
	return &ConnectionMonitor{
		connectionManager: cm,
		healthStatus:      ConnectionHealthStatus{},
		stopChannel:       make(chan struct{}),
		monitoring:        false,
	}
}

// StartMonitoring begins the connection health monitoring process
func (m *ConnectionMonitor) StartMonitoring(ctx context.Context) {
	if m.monitoring {
		return
	}

	m.monitoring = true
	m.startTime = time.Now()
	m.lastStatsLog = time.Now()

	// Start health check routine
	go m.healthCheckRoutine(ctx)

	fmt.Println("🔌 [CONNECTION-MONITOR] Connection pool health monitoring started")
	m.logCurrentStats("Initial Status")
}

// StopMonitoring stops the connection health monitoring
func (m *ConnectionMonitor) StopMonitoring() {
	if !m.monitoring {
		return
	}

	m.monitoring = false
	close(m.stopChannel)
	m.logCurrentStats("Final Status (Shutting Down)")
	fmt.Println("🔌 [CONNECTION-MONITOR] Connection pool health monitoring stopped")
}

// healthCheckRoutine periodically checks connection pool health
func (m *ConnectionMonitor) healthCheckRoutine(ctx context.Context) {
	ticker := time.NewTicker(connectionMonitorInterval)
	defer ticker.Stop()

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("🔌 [CONNECTION-MONITOR] Health check routine panic: %v\n", r)
		}
		fmt.Println("🔌 [CONNECTION-MONITOR] Health check routine stopped")
	}()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("🔌 [CONNECTION-MONITOR] Health check routine context cancelled")
			return
		case <-m.stopChannel:
			fmt.Println("🔌 [CONNECTION-MONITOR] Health check routine stop signal received")
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// performHealthCheck performs a single health check
func (m *ConnectionMonitor) performHealthCheck() {
	stats := m.connectionManager.GetDetailedStats()

	activeConnections := stats["active_connections"].(int)
	maxConnections := stats["max_connections"].(int)
	cacheItems := stats["cache_items"].(int)
	cacheHits := stats["cache_hits"].(int64)
	cacheMisses := stats["cache_misses"].(int64)
	evictions := stats["evictions"].(int64)
	closeErrors := stats["close_errors"].(int64)

	usagePercent := float64(activeConnections) / float64(maxConnections) * 100

	// Determine health level
	var healthLevel HealthLevel
	var message string

	switch {
	case usagePercent >= criticalThresholdPercent:
		healthLevel = HealthLevelCritical
		message = fmt.Sprintf("Connection pool at critical capacity: %.1f%% (%d/%d)",
			usagePercent, activeConnections, maxConnections)
	case usagePercent >= warningThresholdPercent:
		healthLevel = HealthLevelWarning
		message = fmt.Sprintf("Connection pool usage high: %.1f%% (%d/%d)",
			usagePercent, activeConnections, maxConnections)
	default:
		healthLevel = HealthLevelHealthy
		message = fmt.Sprintf("Connection pool healthy: %.1f%% (%d/%d)",
			usagePercent, activeConnections, maxConnections)
	}

	// Check for counter desync (potential bug indicator)
	if activeConnections != cacheItems {
		desyncMessage := fmt.Sprintf(" ⚠️  Counter desync detected: counter=%d, cache=%d",
			activeConnections, cacheItems)
		message += desyncMessage
		if healthLevel == HealthLevelHealthy {
			healthLevel = HealthLevelWarning
		}
	}

	// Update health status
	m.healthMutex.Lock()
	m.healthStatus = ConnectionHealthStatus{
		HealthLevel:       healthLevel,
		ActiveConnections: activeConnections,
		MaxConnections:    maxConnections,
		CacheItems:        cacheItems,
		UsagePercent:      usagePercent,
		CacheHits:         cacheHits,
		CacheMisses:       cacheMisses,
		Evictions:         evictions,
		CloseErrors:       closeErrors,
		LastCheck:         time.Now(),
		Message:           message,
	}
	m.healthMutex.Unlock()

	// Log based on health level
	switch healthLevel {
	case HealthLevelCritical:
		fmt.Printf("🚨 [CONNECTION-MONITOR] CRITICAL: %s\n", message)
		m.logDetailedStats()
	case HealthLevelWarning:
		fmt.Printf("⚠️  [CONNECTION-MONITOR] WARNING: %s\n", message)
	case HealthLevelHealthy:
		// Only log healthy status if enough time has passed
		if time.Since(m.lastStatsLog) >= statsLogInterval {
			m.logCurrentStats("Periodic Health Check")
			m.lastStatsLog = time.Now()
		}
	}

	// Check for high close errors rate
	if closeErrors > 0 {
		totalOps := cacheHits + cacheMisses
		if totalOps > 0 {
			errorRate := float64(closeErrors) / float64(evictions) * 100
			if errorRate > 5 && evictions > 10 { // More than 5% error rate with significant evictions
				fmt.Printf("⚠️  [CONNECTION-MONITOR] High connection close error rate: %.1f%% (%d errors in %d evictions)\n",
					errorRate, closeErrors, evictions)
			}
		}
	}
}

// logCurrentStats logs current connection statistics
func (m *ConnectionMonitor) logCurrentStats(context string) {
	stats := m.connectionManager.GetDetailedStats()
	uptime := time.Since(m.startTime).Round(time.Second)

	fmt.Printf("📊 [CONNECTION-MONITOR] %s\n", context)
	fmt.Printf("   ├─ Active Connections: %d / %d (%.1f%%)\n",
		stats["active_connections"].(int),
		stats["max_connections"].(int),
		float64(stats["active_connections"].(int))/float64(stats["max_connections"].(int))*100)
	fmt.Printf("   ├─ Cache Items: %d\n", stats["cache_items"].(int))
	fmt.Printf("   ├─ Cache Hits: %d\n", stats["cache_hits"].(int64))
	fmt.Printf("   ├─ Cache Misses: %d\n", stats["cache_misses"].(int64))
	fmt.Printf("   ├─ Evictions: %d\n", stats["evictions"].(int64))
	fmt.Printf("   ├─ Close Errors: %d\n", stats["close_errors"].(int64))
	fmt.Printf("   └─ Uptime: %s\n", uptime)
}

// logDetailedStats logs detailed statistics with recommendations
func (m *ConnectionMonitor) logDetailedStats() {
	stats := m.connectionManager.GetDetailedStats()
	uptime := time.Since(m.startTime).Round(time.Second)

	activeConnections := stats["active_connections"].(int)
	maxConnections := stats["max_connections"].(int)
	cacheItems := stats["cache_items"].(int)
	cacheHits := stats["cache_hits"].(int64)
	cacheMisses := stats["cache_misses"].(int64)
	evictions := stats["evictions"].(int64)

	hitRate := float64(0)
	if cacheHits+cacheMisses > 0 {
		hitRate = float64(cacheHits) / float64(cacheHits+cacheMisses) * 100
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Println("📊 [CONNECTION-MONITOR] DETAILED CONNECTION POOL STATUS")
	fmt.Println("═══════════════════════════════════════════════════════════════════")
	fmt.Printf("   Pool Status:          %s\n", m.healthStatus.HealthLevel)
	fmt.Printf("   Active Connections:   %d / %d (%.1f%%)\n", activeConnections, maxConnections, m.healthStatus.UsagePercent)
	fmt.Printf("   Cache Items:          %d\n", cacheItems)
	fmt.Printf("   Cache Hit Rate:       %.1f%% (%d hits, %d misses)\n", hitRate, cacheHits, cacheMisses)
	fmt.Printf("   Total Evictions:      %d\n", evictions)
	fmt.Printf("   Close Errors:         %d\n", stats["close_errors"].(int64))
	fmt.Printf("   Monitoring Uptime:    %s\n", uptime)
	fmt.Println("───────────────────────────────────────────────────────────────────")

	// Recommendations
	if activeConnections != cacheItems {
		fmt.Println("   ⚠️  ISSUE: Counter desync detected!")
		fmt.Println("      - Active connections counter doesn't match cache items")
		fmt.Println("      - This may indicate a bug in connection tracking")
	}

	if m.healthStatus.UsagePercent >= criticalThresholdPercent {
		fmt.Println("   🚨 RECOMMENDATION: Consider the following actions:")
		fmt.Println("      - Increase max_connections limit if hardware permits")
		fmt.Println("      - Review if all connections are necessary")
		fmt.Println("      - Consider restarting the service during low-traffic period")
	}

	if hitRate < 80 && cacheHits+cacheMisses > 100 {
		fmt.Println("   ⚠️  RECOMMENDATION: Low cache hit rate")
		fmt.Println("      - Consider increasing cache TTL")
		fmt.Println("      - Review connection reuse patterns")
	}

	fmt.Println("═══════════════════════════════════════════════════════════════════")
}

// IsHealthy returns true if the connection pool is healthy (under 90% capacity)
func (m *ConnectionMonitor) IsHealthy() bool {
	m.healthMutex.RLock()
	defer m.healthMutex.RUnlock()
	return m.healthStatus.HealthLevel != HealthLevelCritical
}

// GetHealthStatus returns the current health status
func (m *ConnectionMonitor) GetHealthStatus() ConnectionHealthStatus {
	m.healthMutex.RLock()
	defer m.healthMutex.RUnlock()
	return m.healthStatus
}

// GetHealthLevel returns the current health level
func (m *ConnectionMonitor) GetHealthLevel() HealthLevel {
	m.healthMutex.RLock()
	defer m.healthMutex.RUnlock()
	return m.healthStatus.HealthLevel
}

// ForceHealthCheck forces an immediate health check (useful for API endpoints)
func (m *ConnectionMonitor) ForceHealthCheck() ConnectionHealthStatus {
	m.performHealthCheck()
	return m.GetHealthStatus()
}

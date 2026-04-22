package telemetry

import (
	"context"
	"sync"
	"time"

	"github.com/apito-io/engine/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// ConnectionPoolStats is implemented by *database.ConnectionManager for observable pool metrics.
type ConnectionPoolStats interface {
	GetDetailedStats() map[string]interface{}
}

var poolObsOnce sync.Once
var poolObsErr error
var poolActive metric.Int64ObservableGauge
var poolMax metric.Int64ObservableGauge
var poolHits metric.Int64ObservableGauge
var poolMisses metric.Int64ObservableGauge
var poolEvict metric.Int64ObservableGauge
var poolCloseErr metric.Int64ObservableGauge

// RegisterConnectionManagerObservers wires pool stats to OTel observable instruments (no import of database package).
func RegisterConnectionManagerObservers(cfg *models.Config, cm ConnectionPoolStats) {
	if !MetricsEnabled(cfg) || cm == nil {
		return
	}
	poolObsOnce.Do(func() {
		m := otel.GetMeterProvider().Meter(meterName)
		var err error
		poolActive, err = m.Int64ObservableGauge("apito_pool_connections_active",
			metric.WithDescription("Active pooled project DB connections"))
		if err != nil {
			poolObsErr = err
			return
		}
		poolMax, err = m.Int64ObservableGauge("apito_pool_connections_max",
			metric.WithDescription("Maximum pooled connections"))
		if err != nil {
			poolObsErr = err
			return
		}
		poolHits, err = m.Int64ObservableGauge("apito_pool_cache_hits_total",
			metric.WithDescription("Pool cache hits (cumulative)"))
		if err != nil {
			poolObsErr = err
			return
		}
		poolMisses, err = m.Int64ObservableGauge("apito_pool_cache_misses_total",
			metric.WithDescription("Pool cache misses (cumulative)"))
		if err != nil {
			poolObsErr = err
			return
		}
		poolEvict, err = m.Int64ObservableGauge("apito_pool_evictions_total",
			metric.WithDescription("Pool evictions (cumulative)"))
		if err != nil {
			poolObsErr = err
			return
		}
		poolCloseErr, err = m.Int64ObservableGauge("apito_pool_close_errors_total",
			metric.WithDescription("Close errors (cumulative)"))
		if err != nil {
			poolObsErr = err
			return
		}
		_, err = m.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
			st := cm.GetDetailedStats()
			if v, ok := st["active_connections"].(int); ok {
				o.ObserveInt64(poolActive, int64(v))
			}
			if v, ok := st["max_connections"].(int); ok {
				o.ObserveInt64(poolMax, int64(v))
			}
			if v, ok := st["cache_hits"].(int64); ok {
				o.ObserveInt64(poolHits, v)
			}
			if v, ok := st["cache_misses"].(int64); ok {
				o.ObserveInt64(poolMisses, v)
			}
			if v, ok := st["evictions"].(int64); ok {
				o.ObserveInt64(poolEvict, v)
			}
			if v, ok := st["close_errors"].(int64); ok {
				o.ObserveInt64(poolCloseErr, v)
			}
			return nil
		}, poolActive, poolMax, poolHits, poolMisses, poolEvict, poolCloseErr)
		if err != nil {
			poolObsErr = err
		}
	})
}

// ObservePoolCallbackInterval documents expected callback frequency from the metric reader.
const ObservePoolCallbackInterval = 10 * time.Second

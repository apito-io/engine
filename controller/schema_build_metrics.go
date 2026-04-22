package controller

import (
	"context"
	"sync"
	"time"

	"github.com/apito-io/engine/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const schemaBuildMeterName = "apito.engine.public_schema"

var (
	schemaBuildMetricsOnce sync.Once
	schemaBuildTotal       metric.Int64Counter
	schemaBuildDuration    metric.Float64Histogram
	schemaMetricsErr       error
)

func initSchemaBuildMetrics() {
	schemaBuildMetricsOnce.Do(func() {
		m := otel.GetMeterProvider().Meter(schemaBuildMeterName)
		var err error
		schemaBuildTotal, err = m.Int64Counter(
			"schema_build_total",
			metric.WithDescription("Public schema build outcomes: cache hit/miss or error"),
		)
		if err != nil {
			schemaMetricsErr = err
			return
		}
		schemaBuildDuration, err = m.Float64Histogram(
			"schema_build_duration_seconds",
			metric.WithDescription("Duration of publicSchemaBuilder"),
			metric.WithExplicitBucketBoundaries(0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5),
		)
		if err != nil {
			schemaMetricsErr = err
		}
	})
}

func schemaBuildMetricsEnabled(cfg *models.Config) bool {
	return cfg != nil && cfg.SchemaBuildMetrics
}

// recordSchemaBuildOutcome increments schema_build_total when metrics are enabled.
// result is one of: hit, miss, error.
func recordSchemaBuildOutcome(ctx context.Context, cfg *models.Config, result string) {
	if !schemaBuildMetricsEnabled(cfg) {
		return
	}
	initSchemaBuildMetrics()
	if schemaMetricsErr != nil || schemaBuildTotal == nil {
		return
	}
	schemaBuildTotal.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}

func recordSchemaBuildDuration(ctx context.Context, cfg *models.Config, d time.Duration, err error) {
	if !schemaBuildMetricsEnabled(cfg) {
		return
	}
	initSchemaBuildMetrics()
	if schemaMetricsErr != nil || schemaBuildDuration == nil {
		return
	}
	result := "success"
	if err != nil {
		result = "error"
	}
	schemaBuildDuration.Record(ctx, d.Seconds(), metric.WithAttributes(attribute.String("result", result)))
}

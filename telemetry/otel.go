// Package telemetry provides OpenTelemetry metric helpers for the Apito engine.
// Open-core stays Prometheus-client-free; register a MeterProvider at process startup to export metrics (exporter choice is deployment-specific).
package telemetry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/apito-io/engine/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const meterName = "apito.engine"

// MetricsEnabled reports whether apito_* instruments should record (nil-safe).
func MetricsEnabled(cfg *models.Config) bool {
	return cfg != nil && cfg.MetricsEnabled
}

// LatencyBuckets returns standard histogram boundaries for request/operation latency (seconds).
func LatencyBuckets() []float64 {
	return []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
}

var (
	initOnce   sync.Once
	initErr    error
	httpReqTot metric.Int64Counter
	httpDur    metric.Float64Histogram
	httpInflight metric.Int64UpDownCounter
	gqlOpTot   metric.Int64Counter
	gqlOpDur   metric.Float64Histogram
	gqlErrTot  metric.Int64Counter
	restTot    metric.Int64Counter
	restDur    metric.Float64Histogram
	fnExecTot  metric.Int64Counter
	fnExecDur  metric.Float64Histogram
	poolAcqDur metric.Float64Histogram
	dbOpTot    metric.Int64Counter
	dbOpDur    metric.Float64Histogram
	dbDDLTot   metric.Int64Counter
	dbDDLDur   metric.Float64Histogram
	cacheReq   metric.Int64Counter
	kvReqTot   metric.Int64Counter
	kvReqDur   metric.Float64Histogram
	queueEnq   metric.Int64Counter
	queueCon   metric.Int64Counter
	queueDur   metric.Float64Histogram
	sessValTot metric.Int64Counter
	sessValDur metric.Float64Histogram
)

func ensureInstruments() {
	initOnce.Do(func() {
		m := otel.GetMeterProvider().Meter(meterName)
		var err error
		httpReqTot, err = m.Int64Counter("apito_http_requests_total",
			metric.WithDescription("HTTP requests by route pattern"))
		if err != nil {
			initErr = err
			return
		}
		httpDur, err = m.Float64Histogram("apito_http_request_duration_seconds",
			metric.WithDescription("HTTP request latency"),
			metric.WithExplicitBucketBoundaries(LatencyBuckets()...))
		if err != nil {
			initErr = err
			return
		}
		httpInflight, err = m.Int64UpDownCounter("apito_http_request_in_flight",
			metric.WithDescription("In-flight HTTP requests by endpoint pattern"))
		if err != nil {
			initErr = err
			return
		}
		gqlOpTot, err = m.Int64Counter("apito_graphql_operation_total",
			metric.WithDescription("GraphQL operations"))
		if err != nil {
			initErr = err
			return
		}
		gqlOpDur, err = m.Float64Histogram("apito_graphql_operation_duration_seconds",
			metric.WithDescription("GraphQL operation latency"),
			metric.WithExplicitBucketBoundaries(LatencyBuckets()...))
		if err != nil {
			initErr = err
			return
		}
		gqlErrTot, err = m.Int64Counter("apito_graphql_resolver_errors_total",
			metric.WithDescription("GraphQL resolver errors"))
		if err != nil {
			initErr = err
			return
		}
		restTot, err = m.Int64Counter("apito_rest_to_graphql_total",
			metric.WithDescription("REST-to-GraphQL bridge calls"))
		if err != nil {
			initErr = err
			return
		}
		restDur, err = m.Float64Histogram("apito_rest_to_graphql_duration_seconds",
			metric.WithDescription("REST-to-GraphQL latency"),
			metric.WithExplicitBucketBoundaries(LatencyBuckets()...))
		if err != nil {
			initErr = err
			return
		}
		fnExecTot, err = m.Int64Counter("apito_function_execute_total",
			metric.WithDescription("Plugin function executions"))
		if err != nil {
			initErr = err
			return
		}
		fnExecDur, err = m.Float64Histogram("apito_function_execute_duration_seconds",
			metric.WithDescription("Plugin function execution latency"),
			metric.WithExplicitBucketBoundaries(LatencyBuckets()...))
		if err != nil {
			initErr = err
			return
		}
		poolAcqDur, err = m.Float64Histogram("apito_pool_acquire_duration_seconds",
			metric.WithDescription("Connection pool acquire latency"),
			metric.WithExplicitBucketBoundaries(LatencyBuckets()...))
		if err != nil {
			initErr = err
			return
		}
		dbOpTot, err = m.Int64Counter("apito_db_operation_total",
			metric.WithDescription("Project DB high-level operations"))
		if err != nil {
			initErr = err
			return
		}
		dbOpDur, err = m.Float64Histogram("apito_db_operation_duration_seconds",
			metric.WithDescription("Project DB operation latency"),
			metric.WithExplicitBucketBoundaries(LatencyBuckets()...))
		if err != nil {
			initErr = err
			return
		}
		dbDDLTot, err = m.Int64Counter("apito_db_ddl_apply_total",
			metric.WithDescription("DDL-style project DB operations"))
		if err != nil {
			initErr = err
			return
		}
		dbDDLDur, err = m.Float64Histogram("apito_db_ddl_apply_duration_seconds",
			metric.WithDescription("DDL operation latency"),
			metric.WithExplicitBucketBoundaries(LatencyBuckets()...))
		if err != nil {
			initErr = err
			return
		}
		cacheReq, err = m.Int64Counter("apito_cache_requests_total",
			metric.WithDescription("Cache layer requests"))
		if err != nil {
			initErr = err
			return
		}
		kvReqTot, err = m.Int64Counter("apito_kv_requests_total",
			metric.WithDescription("KV store operations"))
		if err != nil {
			initErr = err
			return
		}
		kvReqDur, err = m.Float64Histogram("apito_kv_request_duration_seconds",
			metric.WithDescription("KV operation latency"),
			metric.WithExplicitBucketBoundaries(LatencyBuckets()...))
		if err != nil {
			initErr = err
			return
		}
		queueEnq, err = m.Int64Counter("apito_queue_enqueue_total",
			metric.WithDescription("Queue publish operations"))
		if err != nil {
			initErr = err
			return
		}
		queueCon, err = m.Int64Counter("apito_queue_consume_total",
			metric.WithDescription("Queue consume operations"))
		if err != nil {
			initErr = err
			return
		}
		queueDur, err = m.Float64Histogram("apito_queue_consume_duration_seconds",
			metric.WithDescription("Queue handler latency"),
			metric.WithExplicitBucketBoundaries(LatencyBuckets()...))
		if err != nil {
			initErr = err
			return
		}
		sessValTot, err = m.Int64Counter("apito_session_validate_total",
			metric.WithDescription("Token validation outcomes"))
		if err != nil {
			initErr = err
			return
		}
		sessValDur, err = m.Float64Histogram("apito_session_validate_duration_seconds",
			metric.WithDescription("Token validation latency"),
			metric.WithExplicitBucketBoundaries(LatencyBuckets()...))
		if err != nil {
			initErr = err
			return
		}
	})
}

func resultAttr(err error) attribute.KeyValue {
	if err != nil {
		return attribute.String("result", "error")
	}
	return attribute.String("result", "success")
}

// RecordHTTPRequest records HTTP metrics after a request completes.
func RecordHTTPRequest(ctx context.Context, cfg *models.Config, endpoint, method string, status int, d time.Duration) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil {
		return
	}
	st := status
	if st == 0 {
		st = 200
	}
	attrs := []attribute.KeyValue{
		attribute.String("endpoint", endpoint),
		attribute.String("method", method),
		attribute.Int("status", st),
	}
	httpReqTot.Add(ctx, 1, metric.WithAttributes(attrs...))
	httpDur.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}

// HTTPInFlightDelta adjusts the in-flight gauge (typically +1 before next, -1 in defer).
func HTTPInFlightDelta(ctx context.Context, cfg *models.Config, endpoint string, delta int64) {
	if !MetricsEnabled(cfg) || delta == 0 {
		return
	}
	ensureInstruments()
	if initErr != nil || httpInflight == nil {
		return
	}
	httpInflight.Add(ctx, delta, metric.WithAttributes(attribute.String("endpoint", endpoint)))
}

// EchoRoutePattern returns a bounded route label for metrics (Echo route path or fallback).
func EchoRoutePattern(c interface{ Path() string }) string {
	p := c.Path()
	if p != "" {
		return p
	}
	return "unknown"
}

// RecordGraphQLOperation records GraphQL execution metrics.
func RecordGraphQLOperation(ctx context.Context, cfg *models.Config, surface, operation, opName string, err error, d time.Duration) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil {
		return
	}
	res := "success"
	if err != nil {
		res = "error"
	}
	base := []attribute.KeyValue{
		attribute.String("surface", surface),
		attribute.String("operation", operation),
		attribute.String("op_name", truncateLabel(opName, 128)),
		attribute.String("result", res),
	}
	gqlOpTot.Add(ctx, 1, metric.WithAttributes(base...))
	gqlOpDur.Record(ctx, d.Seconds(), metric.WithAttributes(
		attribute.String("surface", surface),
		attribute.String("operation", operation),
		attribute.String("result", res),
	))
	if err != nil && gqlErrTot != nil {
		gqlErrTot.Add(ctx, 1, metric.WithAttributes(
			attribute.String("surface", surface),
			attribute.String("operation", operation),
			attribute.String("op_name", truncateLabel(opName, 128)),
			attribute.String("code", "resolver_error"),
		))
	}
}

// RecordRESTToGraphQL records REST bridge metrics.
func RecordRESTToGraphQL(ctx context.Context, cfg *models.Config, model, method string, status int, d time.Duration) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("model", truncateLabel(model, 64)),
		attribute.String("method", method),
		attribute.String("status", fmt.Sprintf("%d", status)),
	}
	restTot.Add(ctx, 1, metric.WithAttributes(attrs...))
	restDur.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}

// RecordFunctionExecute records /function execution metrics.
func RecordFunctionExecute(ctx context.Context, cfg *models.Config, plugin string, err error, d time.Duration) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil {
		return
	}
	st := "success"
	if err != nil {
		st = "error"
	}
	attrs := []attribute.KeyValue{
		attribute.String("plugin", truncateLabel(plugin, 128)),
		attribute.String("status", st),
	}
	fnExecTot.Add(ctx, 1, metric.WithAttributes(attrs...))
	fnExecDur.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}

// RecordPoolAcquire records connection pool acquire latency.
func RecordPoolAcquire(ctx context.Context, cfg *models.Config, projectID, engine string, err error, d time.Duration) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil || poolAcqDur == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("project_id", truncateLabel(projectID, 64)),
		attribute.String("engine", truncateLabel(engine, 32)),
		resultAttr(err),
	}
	poolAcqDur.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}

// RecordDBOperation records a non-DDL project DB call.
func RecordDBOperation(ctx context.Context, cfg *models.Config, engine, op string, err error, d time.Duration) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("engine", truncateLabel(engine, 32)),
		attribute.String("op", truncateLabel(op, 32)),
		resultAttr(err),
	}
	dbOpTot.Add(ctx, 1, metric.WithAttributes(attrs...))
	dbOpDur.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}

// RecordDDLApply records DDL-style operations (schema/collection lifecycle).
func RecordDDLApply(ctx context.Context, cfg *models.Config, engine, kind string, err error, d time.Duration) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("engine", truncateLabel(engine, 32)),
		attribute.String("kind", truncateLabel(kind, 32)),
		resultAttr(err),
	}
	dbDDLTot.Add(ctx, 1, metric.WithAttributes(attrs...))
	dbDDLDur.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}

// RecordCacheRequest records cache hit/miss/err.
func RecordCacheRequest(ctx context.Context, cfg *models.Config, layer, result string) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil || cacheReq == nil {
		return
	}
	cacheReq.Add(ctx, 1, metric.WithAttributes(
		attribute.String("layer", truncateLabel(layer, 32)),
		attribute.String("result", truncateLabel(result, 16)),
	))
}

// RecordKVRequest records a KV operation.
func RecordKVRequest(ctx context.Context, cfg *models.Config, op string, err error, d time.Duration) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("op", truncateLabel(op, 32)),
		resultAttr(err),
	}
	kvReqTot.Add(ctx, 1, metric.WithAttributes(attrs...))
	kvReqDur.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}

// RecordQueueEnqueue records a publish to the queue abstraction.
func RecordQueueEnqueue(ctx context.Context, cfg *models.Config, queue string, err error) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil || queueEnq == nil {
		return
	}
	queueEnq.Add(ctx, 1, metric.WithAttributes(
		attribute.String("queue", truncateLabel(queue, 64)),
		resultAttr(err),
	))
}

// RecordQueueConsume records a consume/handling step.
func RecordQueueConsume(ctx context.Context, cfg *models.Config, queue string, err error, d time.Duration) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("queue", truncateLabel(queue, 64)),
		resultAttr(err),
	}
	queueCon.Add(ctx, 1, metric.WithAttributes(attrs...))
	queueDur.Record(ctx, d.Seconds(), metric.WithAttributes(attrs...))
}

// RecordSessionValidate records token validation in middleware.
func RecordSessionValidate(ctx context.Context, cfg *models.Config, result string, d time.Duration) {
	if !MetricsEnabled(cfg) {
		return
	}
	ensureInstruments()
	if initErr != nil {
		return
	}
	sessValTot.Add(ctx, 1, metric.WithAttributes(attribute.String("result", truncateLabel(result, 16))))
	sessValDur.Record(ctx, d.Seconds())
}

func truncateLabel(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

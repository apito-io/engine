//go:build cloudflare

package telemetry

import (
	"context"
	"strconv"
	"time"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

// ConnectionPoolStats is implemented by *database.ConnectionManager for observable pool metrics.
type ConnectionPoolStats interface {
	GetDetailedStats() map[string]interface{}
}

func MetricsEnabled(cfg *models.Config) bool {
	return cfg != nil && cfg.MetricsEnabled
}

func LatencyBuckets() []float64 { return nil }

func EchoMiddleware(_ *models.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc { return next }
}

func StatusString(code int) string {
	if code == 0 {
		return "200"
	}
	return strconv.Itoa(code)
}

func EchoRoutePattern(c interface{ Path() string }) string {
	if p := c.Path(); p != "" {
		return p
	}
	return "unknown"
}

func RegisterConnectionManagerObservers(_ *models.Config, _ ConnectionPoolStats) {}

func WrapProjectDBWithMetrics(_ *models.Config, _ string, inner interfaces.ProjectDBInterface) interfaces.ProjectDBInterface {
	return inner
}

func RecordHTTPRequest(context.Context, *models.Config, string, string, int, time.Duration) {}

func HTTPInFlightDelta(context.Context, *models.Config, string, int64) {}

func RecordGraphQLOperation(context.Context, *models.Config, string, string, string, error, time.Duration) {
}

func RecordRESTToGraphQL(context.Context, *models.Config, string, string, int, time.Duration) {}

func RecordFunctionExecute(context.Context, *models.Config, string, error, time.Duration) {}

func RecordPoolAcquire(context.Context, *models.Config, string, string, error, time.Duration) {}

func RecordDBOperation(context.Context, *models.Config, string, string, error, time.Duration) {}

func RecordDDLApply(context.Context, *models.Config, string, string, error, time.Duration) {}

func RecordCacheRequest(context.Context, *models.Config, string, string) {}

func RecordKVRequest(context.Context, *models.Config, string, error, time.Duration) {}

func RecordQueueEnqueue(context.Context, *models.Config, string, error) {}

func RecordQueueConsume(context.Context, *models.Config, string, error, time.Duration) {}

func RecordSessionValidate(context.Context, *models.Config, string, time.Duration) {}

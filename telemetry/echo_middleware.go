package telemetry

import (
	"strconv"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

// EchoMiddleware returns Echo middleware that records apito_http_* metrics.
// Labels use Echo's route Path() to keep cardinality bounded.
func EchoMiddleware(cfg *models.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !MetricsEnabled(cfg) {
				return next(c)
			}
			ep := EchoRoutePattern(c)
			method := c.Request().Method
			if method == "" {
				method = "GET"
			}
			start := time.Now()
			HTTPInFlightDelta(c.Request().Context(), cfg, ep, 1)
			defer HTTPInFlightDelta(c.Request().Context(), cfg, ep, -1)

			err := next(c)
			status := c.Response().Status
			if status == 0 {
				status = 200
			}
			if err != nil && status < 400 {
				status = 500
			}
			RecordHTTPRequest(c.Request().Context(), cfg, ep, method, status, time.Since(start))
			return err
		}
	}
}

// StatusString returns a three-digit status string for REST metrics.
func StatusString(code int) string {
	if code == 0 {
		return "200"
	}
	return strconv.Itoa(code)
}

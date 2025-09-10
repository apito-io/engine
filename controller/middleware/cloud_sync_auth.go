// Package middleware provides authentication and authorization middleware for HTTP handlers
package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/labstack/echo/v4"
)

// CloudSyncKeyAuth provides authentication middleware for cloud sync operations
func CloudSyncKeyAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get the cloud sync key from environment
			requiredKey := os.Getenv("APITO_CLOUD_SYNC_KEY")
			if requiredKey == "" {
				// If no cloud sync key is configured, reject the request
				return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
					"success": false,
					"error":   "Cloud sync not configured",
					"code":    "CLOUD_SYNC_NOT_CONFIGURED",
				})
			}

			// Get the authorization header
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"error":   "Authorization header required",
					"code":    "MISSING_AUTHORIZATION",
				})
			}

			// Check for Bearer token format
			var providedKey string
			if strings.HasPrefix(authHeader, "Bearer ") {
				providedKey = strings.TrimPrefix(authHeader, "Bearer ")
			} else if strings.HasPrefix(authHeader, "CloudSync ") {
				providedKey = strings.TrimPrefix(authHeader, "CloudSync ")
			} else {
				// Also check X-Cloud-Sync-Key header as alternative
				providedKey = c.Request().Header.Get("X-Cloud-Sync-Key")
				if providedKey == "" {
					return c.JSON(http.StatusUnauthorized, map[string]interface{}{
						"success": false,
						"error":   "Invalid authorization format. Use 'Bearer <key>', 'CloudSync <key>', or 'X-Cloud-Sync-Key' header",
						"code":    "INVALID_AUTH_FORMAT",
					})
				}
			}

			// Validate the key
			if providedKey != requiredKey {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"error":   "Invalid cloud sync key",
					"code":    "INVALID_CLOUD_SYNC_KEY",
				})
			}

			// Key is valid, proceed to next handler
			return next(c)
		}
	}
}

// CloudSyncKeyAuthWithConfig provides configurable authentication middleware
type CloudSyncAuthConfig struct {
	// CloudSyncKey is the required key for authentication
	CloudSyncKey string
	// AllowedHeaders defines which headers can contain the key
	AllowedHeaders []string
	// Skipper defines a function to skip middleware
	Skipper func(echo.Context) bool
}

// DefaultCloudSyncAuthConfig returns default configuration
var DefaultCloudSyncAuthConfig = CloudSyncAuthConfig{
	CloudSyncKey:   os.Getenv("APITO_CLOUD_SYNC_KEY"),
	AllowedHeaders: []string{"Authorization", "X-Cloud-Sync-Key"},
	Skipper:        nil,
}

// CloudSyncKeyAuthWithConfig provides authentication middleware with custom configuration
func CloudSyncKeyAuthWithConfig(config CloudSyncAuthConfig) echo.MiddlewareFunc {
	// Set defaults
	if config.CloudSyncKey == "" {
		config.CloudSyncKey = DefaultCloudSyncAuthConfig.CloudSyncKey
	}
	if config.AllowedHeaders == nil {
		config.AllowedHeaders = DefaultCloudSyncAuthConfig.AllowedHeaders
	}

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip if configured
			if config.Skipper != nil && config.Skipper(c) {
				return next(c)
			}

			// Check if cloud sync is configured
			if config.CloudSyncKey == "" {
				return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
					"success": false,
					"error":   "Cloud sync not configured",
					"code":    "CLOUD_SYNC_NOT_CONFIGURED",
				})
			}

			var providedKey string

			// Check Authorization header with different formats
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader != "" {
				if strings.HasPrefix(authHeader, "Bearer ") {
					providedKey = strings.TrimPrefix(authHeader, "Bearer ")
				} else if strings.HasPrefix(authHeader, "CloudSync ") {
					providedKey = strings.TrimPrefix(authHeader, "CloudSync ")
				}
			}

			// If not found in Authorization, check other allowed headers
			if providedKey == "" {
				for _, header := range config.AllowedHeaders {
					if header != "Authorization" { // Skip Authorization as we already checked it
						if value := c.Request().Header.Get(header); value != "" {
							providedKey = value
							break
						}
					}
				}
			}

			// If still no key found
			if providedKey == "" {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"error":   "Cloud sync key required",
					"code":    "MISSING_CLOUD_SYNC_KEY",
					"hint":    "Provide key in Authorization header (Bearer <key>) or X-Cloud-Sync-Key header",
				})
			}

			// Validate the key
			if providedKey != config.CloudSyncKey {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"success": false,
					"error":   "Invalid cloud sync key",
					"code":    "INVALID_CLOUD_SYNC_KEY",
				})
			}

			// Key is valid, proceed to next handler
			return next(c)
		}
	}
}

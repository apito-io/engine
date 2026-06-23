package middleware

import (
	"net/http"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
)

func CORSMiddleware(cfg *models.Config) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			path := c.Request().URL.Path
			if path == "" {
				path = c.Request().RequestURI
				if i := strings.Index(path, "?"); i >= 0 {
					path = path[:i]
				}
			}

			useTokenFlag := c.Request().Header.Get("X-Use-Cookies")
			accessHeaderRequest := strings.ToLower(c.Request().Header.Get("Access-Control-Request-Headers"))
			needsCredentialedCORS := strings.HasPrefix(path, "/system/") ||
				strings.HasPrefix(path, "/auth/") ||
				useTokenFlag == "true" ||
				strings.Contains(accessHeaderRequest, "x-use-cookies")

			if needsCredentialedCORS {
				origin := strings.TrimSpace(cfg.CORSOrigin)
				if origin == "" {
					origin = c.Request().Header.Get("Origin")
				}
				c.Response().Header().Set("Access-Control-Allow-Origin", origin)
				c.Response().Header().Set("Access-Control-Allow-Credentials", "true")
			} else {
				c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			}

			c.Response().Header().Set("Access-Control-Max-Age", "86400")
			c.Response().Header().Set("Access-Control-Allow-Methods", "POST, PATCH, GET, OPTIONS, PUT, DELETE")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, X-USE-Cookies, X-Apito-Key, X-Apito-Sync-Key, X-Apito-Project-Id, X-Apito-Tenant-ID, X-Connection-Id, X-Fn-Hash, X-Requested-With, Accept-Encoding, X-CSRF-Token, Authorization")
			c.Response().Header().Set("Access-Control-Expose-Headers", "Content-Length")

			if c.Request().Method == "OPTIONS" {
				return c.NoContent(http.StatusNoContent)
			}
			return next(c)
		}
	}
}

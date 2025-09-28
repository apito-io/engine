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

			path := c.Request().RequestURI
			useTokenFlag := c.Request().Header.Get("X-Use-Cookies")
			accessHeaderRequest := c.Request().Header.Get("Access-Control-Request-Headers")
			if strings.HasPrefix(path, "/system/") || strings.HasPrefix(path, "/auth/") || useTokenFlag == "true" || strings.Contains(accessHeaderRequest, "x-use-cookies") { // for all system related calls enforce CORS
				c.Response().Header().Set("Access-Control-Allow-Origin", cfg.CORSOrigin)
			} else { // for public calls ignore it
				c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			}

			c.Response().Header().Set("Access-Control-Max-Age", "86400")
			c.Response().Header().Set("Access-Control-Allow-Methods", "POST, PATCH, GET, OPTIONS, PUT, DELETE")
			c.Response().Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Content-Length, X-USE-Cookies, X-Apito-Key, X-Apito-Sync-Key, X-Apito-Tenant-ID, X-Fn-Hash, X-Requested-With, Accept-Encoding, X-CSRF-Token, Authorization")
			c.Response().Header().Set("Access-Control-Expose-Headers", "Content-Length")
			c.Response().Header().Set("Access-Control-Allow-Credentials", "true")

			if c.Request().Method == "OPTIONS" {
				//c.Response().Header().Set(echo.HeaderAllow, "OPTIONS")
				c.Response().Writer.WriteHeader(http.StatusOK)
			}
			return next(c)
		}
	}
}

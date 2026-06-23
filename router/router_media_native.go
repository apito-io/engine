//go:build !cloudflare

package router

import "github.com/labstack/echo/v4"

func registerMediaRoutes(router *echo.Echo) {
	router.Static("/static/media", "files/storage")
}

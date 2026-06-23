//go:build cloudflare

package router

import "github.com/labstack/echo/v4"

// Media routes are registered via engine/router.RegisterCloudflareOverrides after InitRouter.
func registerMediaRoutes(_ *echo.Echo) {}

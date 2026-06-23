//go:build cloudflare

package controller

import (
	"net/http"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"gitlab.com/apito.io/open_driver/cfd1"
)

// DatabaseCheckCore on Workers only validates cloudflared1; other types are unsupported here.
func DatabaseCheckCore(_ *AuthController, c echo.Context, req *DatabaseRequest) error {
	dbType := strings.ToLower(strings.TrimSpace(req.Type))
	if dbType == cfd1.EngineName || dbType == "d1" || dbType == "cloudflare-d1" {
		return c.JSON(http.StatusOK, map[string]interface{}{"code": http.StatusOK, "message": "cloudflared1 ok"})
	}
	return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: "Invalid database type for Workers build", Code: http.StatusBadRequest})
}

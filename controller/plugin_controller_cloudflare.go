//go:build cloudflare

package controller

import (
	"net/http"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/labstack/echo/v4"
)

// PluginV2Controller stub for Workers (no HashiCorp plugins).
type PluginV2Controller struct {
	Server *resolver.GraphQLServer
}

func NewPluginV2Controller(server *resolver.GraphQLServer) *PluginV2Controller {
	return &PluginV2Controller{Server: server}
}

func pluginNotAvailable(c echo.Context) error {
	return c.JSON(http.StatusNotImplemented, &models.HttpResponse{
		Message: "plugins not available in Workers build",
		Code:    http.StatusNotImplemented,
	})
}

func (pc *PluginV2Controller) CreateOrUpdatePlugin(c echo.Context) error { return pluginNotAvailable(c) }
func (pc *PluginV2Controller) RestartPlugin(c echo.Context) error        { return pluginNotAvailable(c) }
func (pc *PluginV2Controller) StopPlugin(c echo.Context) error           { return pluginNotAvailable(c) }
func (pc *PluginV2Controller) DeletePlugin(c echo.Context) error         { return pluginNotAvailable(c) }
func (pc *PluginV2Controller) GetPlatformInfo(c echo.Context) error      { return pluginNotAvailable(c) }
func (pc *PluginV2Controller) ListPlugins(c echo.Context) error {
	return c.JSON(http.StatusOK, &models.HttpResponse{Message: "ok", Code: http.StatusOK, Body: []any{}})
}
func (pc *PluginV2Controller) GetPluginStatus(c echo.Context) error { return pluginNotAvailable(c) }

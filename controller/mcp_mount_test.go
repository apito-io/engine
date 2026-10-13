package controller

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestMCPSystemCallDisablesCookies(t *testing.T) {
	e := echo.New()
	e.POST("/system/graphql", func(c echo.Context) error {
		if c.Request().Header.Get("X-Use-Cookies") != "false" {
			return c.JSON(http.StatusUnauthorized, map[string]string{"message": "invalid Cookie header. Reload the page"})
		}
		if c.Request().Header.Get("Authorization") != "Bearer apt_internal" {
			return c.NoContent(http.StatusUnauthorized)
		}
		if c.Request().Header.Get("X-Apito-Project-Id") != "suchok_wb8ed" {
			return c.NoContent(http.StatusBadRequest)
		}
		return c.JSON(http.StatusOK, map[string]any{"data": map[string]any{"ok": true}})
	})
	raw, err := (echoSystem{echo: e}).GraphQL(context.Background(), "apt_internal", "suchok_wb8ed", "", `query { ok }`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"ok":true`) {
		t.Fatalf("body = %s", raw)
	}
}

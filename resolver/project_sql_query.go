package resolver

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/apito-io/engine/interfaces"
	"github.com/labstack/echo/v4"
)

// HandleProjectSQLQuery runs one read-only statement on the current project database.
func HandleProjectSQLQuery(c echo.Context, server interface{}) error {
	gs, ok := server.(*GraphQLServer)
	if !ok || gs == nil {
		return c.JSON(503, map[string]string{"error": "project sql is unavailable"})
	}
	cache, err := gs.GetApplicationCache(c)
	if err != nil {
		return err
	}
	if gs.GraphQLExecutor == nil {
		return c.JSON(503, map[string]string{"error": "project database is not ready"})
	}
	driver, err := gs.GraphQLExecutor.GetProjectDriver(cache.Ctx)
	if err != nil {
		return err
	}
	runner, ok := driver.(interfaces.ProjectReadOnlySQL)
	if !ok {
		return c.JSON(400, map[string]string{"error": "this project database does not support SQL exploration"})
	}
	body, err := io.ReadAll(io.LimitReader(c.Request().Body, 1<<20))
	if err != nil {
		return err
	}
	var payload struct {
		SQL string `json:"sql"`
	}
	if err := json.Unmarshal(body, &payload); err != nil || strings.TrimSpace(payload.SQL) == "" {
		return c.JSON(400, map[string]string{"error": "sql is required"})
	}
	cols, rows, err := runner.QueryReadOnly(c.Request().Context(), payload.SQL, 200)
	if err != nil {
		return c.JSON(400, map[string]string{"error": err.Error()})
	}
	return c.JSON(200, map[string]interface{}{
		"columns": cols,
		"rows":    rows,
	})
}

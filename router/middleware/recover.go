package middleware

import (
	"fmt"
	"log"
	"runtime"

	"github.com/apito-io/engine/database"
	"github.com/labstack/echo/v4"
)

// RecoverWithConnectionEvict converts panics into HTTP 500 responses and evicts
// the request-scoped project DB from ConnectionManager so the next request opens a
// fresh driver (critical after Turso/libsql handle corruption).
//
// cmRef is populated after GraphQLServer init; until then eviction is skipped.
func RecoverWithConnectionEvict(cmRef **database.ConnectionManager) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if r := recover(); r != nil {
					evictProjectConnection(cmRef, c)
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}
					stack := make([]byte, 4<<10)
					length := runtime.Stack(stack, false)
					log.Printf("[PANIC RECOVER] %v\n%s", err, stack[:length])
					c.Error(err)
				}
			}()
			return next(c)
		}
	}
}

func evictProjectConnection(cmRef **database.ConnectionManager, c echo.Context) {
	if cmRef == nil || *cmRef == nil {
		return
	}
	connKey := connectionKeyFromContext(c)
	if connKey == "" {
		return
	}
	log.Printf("[PANIC RECOVER] evicting project connection %s", connKey)
	(*cmRef).RemoveConnection(connKey)
}

func connectionKeyFromContext(c echo.Context) string {
	raw := c.Get("project")
	if raw == nil {
		return ""
	}
	projectID, ok := raw.(string)
	if !ok || projectID == "" {
		return ""
	}
	if rawTenant := c.Get("tenant_id"); rawTenant != nil {
		if tenantID, ok := rawTenant.(string); ok && tenantID != "" {
			return projectID + ":" + tenantID
		}
	}
	return projectID
}

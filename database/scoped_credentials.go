package database

import (
	"regexp"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

var scopeIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// ScopedConnectionCacheKey builds the cache key for a per-scope project DB connection.
func ScopedConnectionCacheKey(projectID, scopeKey string) string {
	return projectID + ":" + scopeKey
}

// SanitizeScopeDBSegment returns a safe fragment for database / file names.
func SanitizeScopeDBSegment(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	s = scopeIDSanitizer.ReplaceAllString(s, "_")
	if len(s) > 48 {
		s = s[:48]
	}
	return s
}

// DeriveScopedCredentials copies base project credentials and points at the scope-specific database.
// Base credentials must be registered under projectID (not the composite key).
func DeriveScopedCredentials(base *models.DriverCredentials, projectID, scopeKey string) *models.DriverCredentials {
	if base == nil {
		return nil
	}
	sid := SanitizeScopeDBSegment(scopeKey)
	pid := SanitizeScopeDBSegment(projectID)

	derived := *base
	derived.ProjectID = ScopedConnectionCacheKey(projectID, scopeKey)

	switch base.Engine {
	case _const.PostgreSQLDriver:
		baseDB := base.Database
		if baseDB == "" {
			baseDB = pid
		}
		derived.Database = baseDB + "_" + sid
	case _const.MySQLDriver, _const.MariaDBDriver:
		baseDB := base.Database
		if baseDB == "" {
			baseDB = pid
		}
		derived.Database = baseDB + "_" + sid
	case _const.SQLiteDriver:
		baseFile := base.File
		if baseFile == "" {
			baseFile = pid + ".sqlite"
		}
		baseFile = strings.TrimSuffix(baseFile, ".sqlite")
		derived.File = baseFile + "_" + sid + ".sqlite"
	case "libsql":
		baseDB := base.Database
		if baseDB == "" {
			baseDB = pid
		}
		derived.Database = baseDB + "_" + sid
	default:
		if base.Database != "" {
			derived.Database = base.Database + "_" + sid
		}
	}
	return &derived
}

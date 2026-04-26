package database

import (
	"regexp"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

var scopeIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// scopedLogicalNamePart collapses runs of non-[a-z0-9-] for URL-backed engines where a SQL-style
// "dbname_suffix" string would corrupt a DSN or URL (open-core stays provider-agnostic).
var scopedLogicalNamePart = regexp.MustCompile(`[^a-z0-9-]+`)

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

func looksLikeConnectionURL(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	lower := strings.ToLower(s)
	if strings.HasPrefix(lower, "libsql:") {
		return true
	}
	return strings.Contains(lower, "://")
}

// genericScopeDatabaseIdentifier returns a stable per-scope name when base.Database is URL-shaped.
// It does not encode any hosted-provider rules; the pro layer may replace this before hosted-SQL control-plane calls.
func genericScopeDatabaseIdentifier(projectID, scopeKey string) string {
	pid := strings.ToLower(SanitizeScopeDBSegment(projectID))
	pid = scopedLogicalNamePart.ReplaceAllString(strings.ReplaceAll(pid, "_", "-"), "-")
	pid = strings.Trim(pid, "-")
	if pid == "" {
		pid = "p"
	}
	if len(pid) > 24 {
		pid = pid[:24]
	}
	tid := strings.ToLower(SanitizeScopeDBSegment(scopeKey))
	tid = scopedLogicalNamePart.ReplaceAllString(strings.ReplaceAll(tid, "_", "-"), "-")
	tid = strings.Trim(tid, "-")
	if tid == "" {
		tid = "scope"
	}
	if len(tid) > 24 {
		tid = tid[:24]
	}
	name := "s-" + pid + "-" + tid
	if len(name) > 60 {
		name = name[:60]
	}
	return strings.Trim(name, "-")
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
	default:
		if base.Database != "" {
			if looksLikeConnectionURL(base.Database) {
				derived.Database = genericScopeDatabaseIdentifier(projectID, scopeKey)
			} else {
				derived.Database = base.Database + "_" + sid
			}
		}
	}
	return &derived
}

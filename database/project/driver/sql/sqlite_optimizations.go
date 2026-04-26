package sql

import (
	"context"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/uptrace/bun"
)

// engineIsSQLiteLike reports engines that use SQLite semantics (embedded SQLite, libsql, Turso).
func engineIsSQLiteLike(engine string) bool {
	e := strings.ToLower(strings.TrimSpace(engine))
	switch e {
	case strings.ToLower(_const.SQLiteDriver), "libsql", "turso":
		return true
	default:
		return false
	}
}

func libsqlDSNIsLocalFile(dsn string) bool {
	d := strings.TrimSpace(strings.ToLower(dsn))
	return strings.HasPrefix(d, "file:")
}

// ApplySQLiteConnectionPragmas sets FK enforcement and performance-related PRAGMAs for SQLite/libsql.
// For remote Turso/libsql URLs, WAL/mmap are skipped (not applicable or not supported the same way).
func ApplySQLiteConnectionPragmas(ctx context.Context, orm *bun.DB, engine, dsn string) error {
	if orm == nil || !engineIsSQLiteLike(engine) {
		return nil
	}
	stmts := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA cache_size = -20000;",
	}
	if libsqlDSNIsLocalFile(dsn) {
		stmts = append(stmts,
			"PRAGMA journal_mode = WAL;",
			"PRAGMA synchronous = NORMAL;",
			"PRAGMA mmap_size = 134217728;",
		)
	}
	for _, s := range stmts {
		if _, err := orm.NewRaw(s).Exec(ctx); err != nil {
			// Remote libsql may ignore some PRAGMAs; continue best-effort.
			continue
		}
	}
	return nil
}

// RunSQLiteLikePostDDL runs ANALYZE on SQLite-like engines after DDL changes.
func RunSQLiteLikePostDDL(ctx context.Context, S *SQLDriver) error {
	if S == nil || S.ORM == nil || S.DriverCredential == nil || !engineIsSQLiteLike(S.DriverCredential.Engine) {
		return nil
	}
	_, _ = S.ORM.NewRaw("ANALYZE").Exec(ctx)
	return nil
}

// RunAnalyzeAfterIndexDDL refreshes planner statistics after CREATE INDEX on SQLite-like engines.
func RunAnalyzeAfterIndexDDL(ctx context.Context, S *SQLDriver) {
	_ = RunSQLiteLikePostDDL(ctx, S)
}

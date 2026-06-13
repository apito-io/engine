package sqlcommon

import (
	"context"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/uptrace/bun"
)

// EngineIsSQLiteLike reports engines that use SQLite semantics (embedded SQLite, libsql, Turso).
func EngineIsSQLiteLike(engine string) bool {
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
func ApplySQLiteConnectionPragmas(ctx context.Context, orm *bun.DB, engine, dsn string) error {
	if orm == nil || !EngineIsSQLiteLike(engine) {
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
			continue
		}
	}
	return nil
}

// ApplyLitestreamConnectionPragmas configures SQLite for embedded Litestream local files.
func ApplyLitestreamConnectionPragmas(ctx context.Context, orm *bun.DB, engine string) error {
	if orm == nil || !EngineIsSQLiteLike(engine) {
		return nil
	}
	stmts := []string{
		"PRAGMA foreign_keys = ON;",
		"PRAGMA temp_store = MEMORY;",
		"PRAGMA cache_size = -20000;",
	}
	for _, s := range stmts {
		if _, err := orm.NewRaw(s).Exec(ctx); err != nil {
			continue
		}
	}
	return nil
}

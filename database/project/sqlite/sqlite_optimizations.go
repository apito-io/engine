package sqlite

import (
	"context"
	"strings"
	"time"

	"github.com/apito-io/engine/database/project/sqlcommon"
	"github.com/uptrace/bun"
)

func isLibsqlTransientHTTPErr(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	for _, code := range []string{"error code 502", "error code 503", "error code 504", "error code 429", "stream_expired"} {
		if strings.Contains(s, code) {
			return true
		}
	}
	return false
}

func (d *Driver) preferDDLRunInTx() bool {
	if d != nil && d.IsRemoteSQLiteLikeTurso() {
		return false
	}
	return true
}

func (d *Driver) runDDLBatch(ctx context.Context, fn func(bun.IDB) error) error {
	if d == nil || d.Dialect == nil {
		return fn(d.ORM)
	}
	return d.Dialect.RunDDLBatch(ctx, &d.Driver, fn)
}

func (d *Driver) execRawDDL(ctx context.Context, db bun.IDB, query string) error {
	if d == nil || db == nil {
		return nil
	}
	if !d.IsRemoteSQLiteLikeTurso() {
		_, err := db.NewRaw(query).Exec(ctx)
		return err
	}
	const maxAttempts = 4
	var last error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		_, last = db.NewRaw(query).Exec(ctx)
		if last == nil {
			return nil
		}
		if !isLibsqlTransientHTTPErr(last) || attempt == maxAttempts {
			return last
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
	return last
}

func (d *Driver) IsRemoteSQLiteLikeTurso() bool {
	if d == nil || d.Dialect == nil {
		return false
	}
	return d.Dialect.IsRemoteSQLiteLikeTurso(&d.Driver)
}

func RunSQLiteLikePostDDL(ctx context.Context, d *Driver) error {
	if d == nil || d.ORM == nil || d.DriverCredential == nil || !sqlcommon.EngineIsSQLiteLike(d.DriverCredential.Engine) {
		return nil
	}
	_, _ = d.ORM.NewRaw("ANALYZE").Exec(ctx)
	return nil
}

func RunAnalyzeAfterIndexDDL(ctx context.Context, d *Driver) {
	_ = RunSQLiteLikePostDDL(ctx, d)
}

// ApplyLitestreamConnectionPragmas configures SQLite for embedded Litestream local files.
func ApplyLitestreamConnectionPragmas(ctx context.Context, orm *bun.DB, engine string) error {
	return sqlcommon.ApplyLitestreamConnectionPragmas(ctx, orm, engine)
}

// ApplyTursoSyncConnectionPragmas is deprecated; use ApplyLitestreamConnectionPragmas.
func ApplyTursoSyncConnectionPragmas(ctx context.Context, orm *bun.DB, engine string) error {
	return ApplyLitestreamConnectionPragmas(ctx, orm, engine)
}

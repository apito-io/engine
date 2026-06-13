package sqlite

import (
	"context"
	"strings"
	"time"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/project/sqlcommon"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

type sqliteDialect struct{}

func (sqliteDialect) Engine() string { return _const.SQLiteDriver }

func (sqliteDialect) QuoteIdent(name string) string {
	s := strings.ReplaceAll(name, "`", "``")
	return "`" + s + "`"
}

func (sqliteDialect) SupportsTransactionalDDL() bool { return true }

func (sqliteDialect) IsSQLiteLike() bool { return true }

func (sqliteDialect) IsRemoteSQLiteLikeTurso(d *sqlcommon.Driver) bool {
	if d == nil || d.DriverCredential == nil {
		return false
	}
	if !sqlcommon.EngineIsSQLiteLike(d.DriverCredential.Engine) {
		return false
	}
	db := strings.ToLower(strings.TrimSpace(d.DriverCredential.Database))
	if strings.HasPrefix(db, "file:") {
		return false
	}
	if strings.HasPrefix(db, "libsql://") || strings.HasPrefix(db, "https://") || strings.HasPrefix(db, "http://") {
		return true
	}
	org := strings.TrimSpace(d.DriverCredential.DatabaseDir)
	return org != "" && !strings.HasPrefix(strings.ToLower(org), "file:")
}

func (d sqliteDialect) PreferDDLRunInTx(drv *sqlcommon.Driver) bool {
	if drv != nil && d.IsRemoteSQLiteLikeTurso(drv) {
		return false
	}
	return true
}

func (d sqliteDialect) RunDDLBatch(ctx context.Context, drv *sqlcommon.Driver, fn func(bun.IDB) error) error {
	if d.PreferDDLRunInTx(drv) {
		return drv.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			return fn(tx)
		})
	}
	if !d.IsRemoteSQLiteLikeTurso(drv) {
		return fn(drv.ORM)
	}
	const maxAttempts = 4
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = fn(drv.ORM)
		if err == nil {
			return nil
		}
		if !isLibsqlTransientHTTPErr(err) || attempt == maxAttempts {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Duration(attempt) * time.Second):
		}
	}
	return err
}

func (sqliteDialect) RootResolverQueryBuilder(conf *models.Config, param *models.CommonSystemParams, returnCount bool) (string, []interface{}, error) {
	return RootResolverQueryBuilder(conf, param, returnCount)
}

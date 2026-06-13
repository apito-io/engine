package postgres

import (
	"context"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/project/sqlcommon"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

type postgresDialect struct{}

func (postgresDialect) Engine() string { return _const.PostgreSQLDriver }

func (postgresDialect) QuoteIdent(name string) string { return sqlcommon.QuotePGIdent(name) }

func (postgresDialect) SupportsTransactionalDDL() bool { return true }

func (postgresDialect) IsSQLiteLike() bool { return false }

func (postgresDialect) IsRemoteSQLiteLikeTurso(*sqlcommon.Driver) bool { return false }

func (postgresDialect) PreferDDLRunInTx(*sqlcommon.Driver) bool { return true }

func (postgresDialect) RunDDLBatch(ctx context.Context, drv *sqlcommon.Driver, fn func(bun.IDB) error) error {
	return drv.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return fn(tx)
	})
}

func (postgresDialect) RootResolverQueryBuilder(conf *models.Config, param *models.CommonSystemParams, returnCount bool) (string, []interface{}, error) {
	return RootResolverQueryBuilder(conf, param, returnCount)
}

package mysql

import (
	"context"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/project/sqlcommon"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

type mysqlDialect struct{}

func (mysqlDialect) Engine() string { return _const.MySQLDriver }

func (mysqlDialect) QuoteIdent(name string) string { return sqlcommon.QuoteMySQLIdent(name) }

func (mysqlDialect) SupportsTransactionalDDL() bool { return false }

func (mysqlDialect) IsSQLiteLike() bool { return false }

func (mysqlDialect) IsRemoteSQLiteLikeTurso(*sqlcommon.Driver) bool { return false }

func (mysqlDialect) PreferDDLRunInTx(*sqlcommon.Driver) bool { return false }

func (mysqlDialect) RunDDLBatch(ctx context.Context, drv *sqlcommon.Driver, fn func(bun.IDB) error) error {
	return fn(drv.ORM)
}

func (mysqlDialect) RootResolverQueryBuilder(conf *models.Config, param *models.CommonSystemParams, returnCount bool) (string, []interface{}, error) {
	return RootResolverQueryBuilder(conf, param, returnCount)
}

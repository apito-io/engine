package mariadb

import (
	"context"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/project/sqlcommon"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

type mariadbDialect struct{}

func (mariadbDialect) Engine() string { return _const.MariaDBDriver }

func (mariadbDialect) QuoteIdent(name string) string { return sqlcommon.QuoteMySQLIdent(name) }

func (mariadbDialect) SupportsTransactionalDDL() bool { return false }

func (mariadbDialect) IsSQLiteLike() bool { return false }

func (mariadbDialect) IsRemoteSQLiteLikeTurso(*sqlcommon.Driver) bool { return false }

func (mariadbDialect) PreferDDLRunInTx(*sqlcommon.Driver) bool { return false }

func (mariadbDialect) RunDDLBatch(ctx context.Context, drv *sqlcommon.Driver, fn func(bun.IDB) error) error {
	return fn(drv.ORM)
}

func (mariadbDialect) RootResolverQueryBuilder(conf *models.Config, param *models.CommonSystemParams, returnCount bool) (string, []interface{}, error) {
	return RootResolverQueryBuilder(conf, param, returnCount)
}

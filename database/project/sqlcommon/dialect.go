package sqlcommon

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

// Dialect captures engine-specific SQL behavior for the shared driver.
type Dialect interface {
	Engine() string
	QuoteIdent(name string) string
	SupportsTransactionalDDL() bool
	IsSQLiteLike() bool
	IsRemoteSQLiteLikeTurso(d *Driver) bool
	PreferDDLRunInTx(d *Driver) bool
	RunDDLBatch(ctx context.Context, d *Driver, fn func(bun.IDB) error) error
	RootResolverQueryBuilder(conf *models.Config, param *models.CommonSystemParams, returnCount bool) (string, []interface{}, error)
}

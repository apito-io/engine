package mariadb

import (
	"context"

	"github.com/uptrace/bun"
)

func (d *Driver) preferDDLRunInTx() bool {
	if d == nil || d.Dialect == nil {
		return false
	}
	return d.Dialect.PreferDDLRunInTx(&d.Driver)
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
	_, err := db.NewRaw(query).Exec(ctx)
	return err
}

func (d *Driver) IsRemoteSQLiteLikeTurso() bool { return false }

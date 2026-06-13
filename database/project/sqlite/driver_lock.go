package sqlite

import (
	"context"

	"github.com/uptrace/bun"
)

func (d *Driver) lockSQLiteWrite() bool { return d.LockSQLiteWrite() }

func (d *Driver) unlockSQLiteWrite() { d.UnlockSQLiteWrite() }

func (d *Driver) runInTxOrBypass(ctx context.Context, fn func(ctx context.Context, tx bun.IDB) error) error {
	return d.RunInTxOrBypass(ctx, fn)
}

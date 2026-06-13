package sqlcommon

import (
	"context"
	"sync"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

// SyncLocker optionally serializes foreground DB operations (legacy hook).
type SyncLocker interface {
	LockForTx()
	UnlockForTx()
}

// Base holds shared Bun ORM state for all SQL project drivers.
type Base struct {
	Conf             *models.Config
	ORM              *bun.DB
	DriverCredential *models.DriverCredentials
	SyncLock         SyncLocker
	sqliteWriteMu    sync.Mutex
	Dialect          Dialect
}

func (b *Base) lockSQLiteWrite() bool {
	if b == nil || b.DriverCredential == nil || b.Dialect == nil || !b.Dialect.IsSQLiteLike() {
		return false
	}
	b.sqliteWriteMu.Lock()
	if b.SyncLock != nil {
		b.SyncLock.LockForTx()
	}
	return true
}

// LockSQLiteWrite acquires SQLite-like write serialization when applicable.
func (b *Base) LockSQLiteWrite() bool { return b.lockSQLiteWrite() }

func (b *Base) unlockSQLiteWrite() {
	if b.SyncLock != nil {
		b.SyncLock.UnlockForTx()
	}
	b.sqliteWriteMu.Unlock()
}

// UnlockSQLiteWrite releases SQLite-like write serialization.
func (b *Base) UnlockSQLiteWrite() { b.unlockSQLiteWrite() }

// WithWriteLock runs fn while holding SQLite-like write serialization when applicable.
func (b *Base) WithWriteLock(fn func() error) error {
	if locked := b.lockSQLiteWrite(); locked {
		defer b.unlockSQLiteWrite()
	}
	return fn()
}

// RunInTxOrBypass runs fn on the bare connection for SQLite-like engines, else inside a transaction.
func (b *Base) RunInTxOrBypass(ctx context.Context, fn func(ctx context.Context, tx bun.IDB) error) error {
	if b.DriverCredential != nil && b.Dialect != nil && b.Dialect.IsSQLiteLike() {
		return fn(ctx, b.ORM)
	}
	return b.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return fn(ctx, tx)
	})
}

func (b *Base) runInTxOrBypass(ctx context.Context, fn func(ctx context.Context, tx bun.IDB) error) error {
	return b.RunInTxOrBypass(ctx, fn)
}

// Driver is the shared SQL project driver implementation used by engine packages.
type Driver struct {
	Base
}

func (d *Driver) Engine() string {
	if d == nil || d.DriverCredential == nil {
		return ""
	}
	return d.DriverCredential.Engine
}

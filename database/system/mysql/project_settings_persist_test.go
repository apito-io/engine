package mysql

import (
	"context"
	"database/sql"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestEnsureProjectSettingsSQLColumns(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := testSystemDriver(db, &models.Config{SystemDatabaseEngine: "coredb"}, nil)
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))
	require.NoError(t, ensureProjectSettingsSQLColumns(ctx, db))
}

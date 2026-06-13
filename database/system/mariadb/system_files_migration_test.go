package mariadb

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

func TestSystemMigrationDoesNotCreateFilesTable(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := testSystemDriver(db, &models.Config{SystemDatabaseEngine: "coredb"}, nil)
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))

	var n int
	err = db.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='files'`).Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n, "system DB migration must not create a files table")
}

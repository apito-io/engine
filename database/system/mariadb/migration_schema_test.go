package mariadb

import (
	"github.com/apito-io/engine/database/system/sqlcommon"
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestRunMigrationModelsBunSchema(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	sqlcommon.RegisterSystemSQLSchemaModels(db)
	ctx := context.Background()
	for _, m := range sqlcommon.SystemSQLSchemaModels() {
		_, err := db.NewCreateTable().IfNotExists().Model(m).Exec(ctx)
		require.NoError(t, err, "%T", m)
	}
	var createSQL string
	err = sqldb.QueryRowContext(ctx, `SELECT sql FROM sqlite_master WHERE type='table' AND name='model_types'`).Scan(&createSQL)
	require.NoError(t, err)
	require.Contains(t, createSQL, `"project_id"`)
	require.Contains(t, createSQL, `"name"`)
	require.Contains(t, createSQL, `PRIMARY KEY ("project_id", "name")`)
}

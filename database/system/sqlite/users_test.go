package sqlite

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"database/sql"
)

func TestCreateUserRoundTripSQLite(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bunNewSQLite(sqldb)
	drv := testSystemDriver(db, &models.Config{
			SystemDatabaseEngine: "coredb",
		}, nil)
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))

	row := &models.User{
		ID:        "u-test-1",
		ProjectID: "proj-1",
		Username:  "u_test_1",
		Email:     "a@example.com",
		Secret:    "hash",
		Role:      "none",
		Provider:  models.UserProviderLocal,
		Status:    models.UserStatusActive,
	}
	created, err := drv.CreateUser(ctx, row)
	require.NoError(t, err)
	require.NotNil(t, created)
	require.Equal(t, row.ID, created.ID)

	got, err := drv.GetUser(ctx, "proj-1", "u-test-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "a@example.com", got.Email)
}

func bunNewSQLite(sqldb *sql.DB) *bun.DB {
	return bun.NewDB(sqldb, sqlitedialect.New())
}

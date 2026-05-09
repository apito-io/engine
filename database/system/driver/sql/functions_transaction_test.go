package sql

import (
	"context"
	"database/sql"
	"testing"

	"github.com/apito-io/engine/database/system/bootstrapmeta"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// SQLite triggers abort the second INSERT inside RunInTx so we verify the whole PersistProjectModelTypes call rolls back.

func TestPersistProjectModelTypes_secondInsertFails_rollsBackDeleteAndFirstInsert(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := &SystemSQLDriver{
		ORM: db,
		Conf: &models.Config{
			SystemDatabaseEngine: "coredb",
		},
		DriverCredential: nil,
	}
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))
	require.NoError(t, drv.EnsureSystemBootstrap(ctx))

	pid := bootstrapmeta.StarterProjectID

	_, err = db.NewRaw(`
DROP TRIGGER IF EXISTS test_abort_second_model_types_insert;
CREATE TRIGGER test_abort_second_model_types_insert
BEFORE INSERT ON model_types
FOR EACH ROW
WHEN (
  (SELECT COUNT(*) FROM model_types WHERE project_id = NEW.project_id) >= 1
)
BEGIN
  SELECT RAISE(ABORT, 'test_second_model_types_insert');
END;
`).Exec(ctx)
	require.NoError(t, err)

	_, err = db.NewInsert().Model(&models.ModelType{
		ProjectID: pid,
		Name:      "legacy_row",
	}).Exec(ctx)
	require.NoError(t, err)

	err = drv.PersistProjectModelTypes(ctx, pid, []*models.ModelType{
		{Name: "first_new"},
		{Name: "second_new"},
	})
	require.Error(t, err)

	var nLegacy int
	err = db.NewSelect().ColumnExpr("count(*)").Table("model_types").
		Where("project_id = ? AND name = ?", pid, "legacy_row").
		Scan(ctx, &nLegacy)
	require.NoError(t, err)
	require.Equal(t, 1, nLegacy)

	var nFirst int
	err = db.NewSelect().ColumnExpr("count(*)").Table("model_types").
		Where("project_id = ? AND name = ?", pid, "first_new").
		Scan(ctx, &nFirst)
	require.NoError(t, err)
	require.Equal(t, 0, nFirst)
}

func TestSyncProjectTokens_secondInsertFails_restoresPriorTokens(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := &SystemSQLDriver{
		ORM: db,
		Conf: &models.Config{
			SystemDatabaseEngine: "coredb",
		},
		DriverCredential: nil,
	}
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))
	require.NoError(t, drv.EnsureSystemBootstrap(ctx))

	pid := bootstrapmeta.StarterProjectID

	_, err = db.NewRaw(`
DROP TRIGGER IF EXISTS test_abort_second_project_tokens_insert;
CREATE TRIGGER test_abort_second_project_tokens_insert
BEFORE INSERT ON project_tokens
FOR EACH ROW
WHEN (
  (SELECT COUNT(*) FROM project_tokens WHERE project_id = NEW.project_id) >= 1
)
BEGIN
  SELECT RAISE(ABORT, 'test_second_project_tokens_insert');
END;
`).Exec(ctx)
	require.NoError(t, err)

	_, err = db.NewInsert().Model(&models.ProjectToken{
		ProjectID: pid,
		Name:      "keep",
		Token:     "secret-keep",
	}).Exec(ctx)
	require.NoError(t, err)

	proj := &models.Project{
		ID: pid,
		Tokens: []*models.ProjectToken{
			{Name: "t1", Token: "a"},
			{Name: "t2", Token: "b"},
		},
	}

	err = drv.syncProjectTokens(ctx, proj)
	require.Error(t, err)

	var n int
	err = db.NewSelect().ColumnExpr("count(*)").Table("project_tokens").
		Where("project_id = ?", pid).
		Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	var tok string
	err = db.NewSelect().Column("token").Table("project_tokens").
		Where("project_id = ? AND name = ?", pid, "keep").
		Scan(ctx, &tok)
	require.NoError(t, err)
	require.Equal(t, "secret-keep", tok)
}

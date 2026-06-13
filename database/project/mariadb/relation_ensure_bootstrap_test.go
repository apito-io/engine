package mariadb

import (
	"context"
	"database/sql"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// rosna-like schema fragment: 1:1, 1:N, and M:N on food.
func testFoodRelationSchema() []*models.ModelType {
	return []*models.ModelType{
		{
			Name: "food",
			Connections: []*models.ConnectionType{
				{Model: "stock", Relation: "has_one", Type: "forward"},
				{Model: "food_category", Relation: "has_one", Type: "forward"},
				{Model: "addon", Relation: "has_one", Type: "forward"},
				{Model: "food_order", Relation: "has_many", Type: "forward"},
			},
		},
		{Name: "stock", Connections: []*models.ConnectionType{
			{Model: "food", Relation: "has_one", Type: "backward"},
		}},
		{Name: "food_category", Connections: []*models.ConnectionType{
			{Model: "food", Relation: "has_many", Type: "backward"},
		}},
		{Name: "addon", Connections: []*models.ConnectionType{
			{Model: "food", Relation: "has_one", Type: "backward"},
		}},
		{
			Name: "food_order",
			Connections: []*models.ConnectionType{
				{Model: "food", Relation: "has_many", Type: "backward"},
				{Model: "employee", Relation: "has_one", Type: "forward", KnownAs: "chef"},
			},
		},
		{
			Name: "employee",
			Connections: []*models.ConnectionType{
				{Model: "food_order", Relation: "has_many", Type: "backward", KnownAs: "chef"},
			},
		},
	}
}

func TestEnsureRelationArtifactsFromSchema_bootstrapCoverage(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE food (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE stock (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE food_category (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE addon (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE food_order (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE employee (id VARCHAR(36) NOT NULL PRIMARY KEY);
`).Exec(ctx)
	require.NoError(t, err)

	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	require.NoError(t, drv.EnsureRelationArtifactsFromSchema(ctx, testFoodRelationSchema()))

	// M:N pivot is created today (picture 3).
	assertTable(t, db, ctx, "food_food_order", true)
	// 1:1 FK columns are missing until EnsureRelationArtifactsFromSchema handles has_one pairs (picture 1 vs 2).
	assertColumn(t, db, ctx, "food", "stock_id", true)
	assertColumn(t, db, ctx, "food", "food_category_id", true) // has_one forward on food, has_many backward on category
	assertColumn(t, db, ctx, "food", "addon_id", true)
	assertColumn(t, db, ctx, "food_order", "employee_as_chef_id", true)
}

// category declares forward has_many; food declares backward has_one (common console pattern).
func TestEnsureRelationArtifactsFromSchema_categoryForwardFoodBackward(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewRaw(`
CREATE TABLE food (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE food_category (id VARCHAR(36) NOT NULL PRIMARY KEY);
`).Exec(ctx)
	require.NoError(t, err)
	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	schema := []*models.ModelType{
		{Name: "food_category", Connections: []*models.ConnectionType{
			{Model: "food", Relation: "has_many", Type: "forward"},
		}},
		{Name: "food", Connections: []*models.ConnectionType{
			{Model: "food_category", Relation: "has_one", Type: "backward"},
		}},
	}
	require.NoError(t, drv.EnsureRelationArtifactsFromSchema(ctx, schema))
	assertColumn(t, db, ctx, "food", "food_category_id", true)
	assertColumn(t, db, ctx, "food_category", "food_id", false)
}

func TestEnsureRelationArtifactsFromSchema_manyToManyAddonPivot(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewRaw(`
CREATE TABLE food (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE addon (id VARCHAR(36) NOT NULL PRIMARY KEY);
`).Exec(ctx)
	require.NoError(t, err)
	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	schema := []*models.ModelType{
		{Name: "food", Connections: []*models.ConnectionType{
			{Model: "addon", Relation: "has_many", Type: "forward"},
		}},
		{Name: "addon", Connections: []*models.ConnectionType{
			{Model: "food", Relation: "has_many", Type: "backward"},
		}},
	}
	require.NoError(t, drv.EnsureRelationArtifactsFromSchema(ctx, schema))
	assertTable(t, db, ctx, "addon_food", true)
}

// ledger.food_order_id may already exist from EnsureModelUserFieldColumns (schema Fields) before relation bootstrap.
func TestEnsureRelationArtifactsFromSchema_idempotentWhenFKColumnExists(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewRaw(`
CREATE TABLE food_order (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE ledger (id VARCHAR(36) NOT NULL PRIMARY KEY, food_order_id VARCHAR(36));
`).Exec(ctx)
	require.NoError(t, err)
	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	schema := []*models.ModelType{
		{Name: "food_order", Connections: []*models.ConnectionType{
			{Model: "ledger", Relation: "has_many", Type: "forward"},
		}},
		{Name: "ledger", Connections: []*models.ConnectionType{
			{Model: "food_order", Relation: "has_one", Type: "backward"},
		}},
	}
	require.NoError(t, drv.EnsureRelationArtifactsFromSchema(ctx, schema))
	assertColumn(t, db, ctx, "ledger", "food_order_id", true)
}

func TestEnsureRelationArtifactsFromSchema_foodOrderHasOneForward(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	_, err = db.NewRaw(`
CREATE TABLE food_order (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE employee (id VARCHAR(36) NOT NULL PRIMARY KEY);
`).Exec(ctx)
	require.NoError(t, err)
	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	schema := []*models.ModelType{
		{Name: "food_order", Connections: []*models.ConnectionType{
			{Model: "employee", Relation: "has_one", Type: "forward", KnownAs: "chef"},
		}},
		{Name: "employee", Connections: []*models.ConnectionType{
			{Model: "food_order", Relation: "has_many", Type: "backward", KnownAs: "chef"},
		}},
	}
	require.NoError(t, drv.EnsureRelationArtifactsFromSchema(ctx, schema))
	assertColumn(t, db, ctx, "food_order", "employee_as_chef_id", true)
}

func assertColumn(t *testing.T, db *bun.DB, ctx context.Context, table, col string, want bool) {
	t.Helper()
	var n int
	err := db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?`, table, col).Scan(ctx, &n)
	require.NoError(t, err)
	if want {
		require.Equal(t, 1, n, "expected column %s.%s", table, col)
	} else {
		require.Equal(t, 0, n, "unexpected column %s.%s", table, col)
	}
}

func assertTable(t *testing.T, db *bun.DB, ctx context.Context, name string, want bool) {
	t.Helper()
	var n int
	err := db.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name).Scan(ctx, &n)
	require.NoError(t, err)
	if want {
		require.Equal(t, 1, n, "expected table %s", name)
	} else {
		require.Equal(t, 0, n, "unexpected table %s", name)
	}
}

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

func TestConnectDisconnectIsOneToOne(t *testing.T) {
	cdp := &models.ConnectDisconnectParam{
		ForwardConnectionType:  &models.ConnectionType{Relation: "has_one"},
		BackwardConnectionType: &models.ConnectionType{Relation: "has_one"},
	}
	require.True(t, connectDisconnectIsOneToOne(cdp))
	cdp.BackwardConnectionType.Relation = "has_many"
	require.False(t, connectDisconnectIsOneToOne(cdp))
	require.False(t, connectDisconnectIsOneToOne(nil))
}

func TestAddRelationFieldsOneToOne_sqliteAddsDualFKColumns(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE stock (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE food (id VARCHAR(36) NOT NULL PRIMARY KEY);
`).Exec(ctx)
	require.NoError(t, err)

	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	from := &models.ConnectionType{Model: "stock", Relation: "has_one", Type: "forward"}
	to := &models.ConnectionType{Model: "food", Relation: "has_one", Type: "backward"}
	require.NoError(t, drv.AddRelationFields(ctx, from, to))

	var n int
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('stock') WHERE name = ?`, "food_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('food') WHERE name = ?`, "stock_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	require.NoError(t, drv.DeleteRelationDocuments(ctx, "p1", from, to))

	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('stock') WHERE name = ?`, "food_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n)
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('food') WHERE name = ?`, "stock_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	err = db.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, "idx_stock_food_id_unique").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n)
	err = db.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, "idx_food_stock_id_unique").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

func TestAddRelationFieldsHasManyHasOne_knownAsCreatesDistinctFKs(t *testing.T) {
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

	waiterFrom := &models.ConnectionType{Model: "food_order", Relation: "has_many", Type: "forward", KnownAs: "waiter"}
	waiterTo := &models.ConnectionType{Model: "employee", Relation: "has_one", Type: "backward"}
	require.NoError(t, drv.AddRelationFields(ctx, waiterFrom, waiterTo))

	chefFrom := &models.ConnectionType{Model: "food_order", Relation: "has_many", Type: "forward", KnownAs: "chef"}
	chefTo := &models.ConnectionType{Model: "employee", Relation: "has_one", Type: "backward"}
	require.NoError(t, drv.AddRelationFields(ctx, chefFrom, chefTo))

	var n int
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('food_order') WHERE name = ?`, "employee_as_waiter_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('food_order') WHERE name = ?`, "employee_as_chef_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

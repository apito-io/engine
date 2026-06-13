package sqlite

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

// Regression: SQLite DROP COLUMN on a table with Apito row-count triggers that reference tenant_id
// must drop triggers first; otherwise SQLite fails rebuilding the table ("unknown column ... in foreign key definition").
func TestDeleteRelationDocuments_hasManyHasOne_withRowCountTriggers(t *testing.T) {
	t.Setenv(envTursoCounterTriggers, "true")
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE tenant (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE employee (id VARCHAR(36) NOT NULL PRIMARY KEY);
`).Exec(ctx)
	require.NoError(t, err)

	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})

	// Forward on tenant lists peer employee (has_many); backward on employee lists peer tenant (has_one).
	from := &models.ConnectionType{Model: "employee", Relation: "has_many", Type: "forward"}
	to := &models.ConnectionType{Model: "tenant", Relation: "has_one", Type: "backward"}
	require.NoError(t, drv.AddRelationFields(ctx, from, to))

	var n int
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('employee') WHERE name = ?`, "tenant_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	require.NoError(t, drv.installRowCountTriggersForModel(ctx, &models.ModelType{Name: "employee"}))

	require.NoError(t, drv.DeleteRelationDocuments(ctx, "p1", from, to))

	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('employee') WHERE name = ?`, "tenant_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	err = db.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'trigger' AND tbl_name = 'employee'`).Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 2, n, "row-count triggers should be recreated after DROP COLUMN")
}

// Same failure mode when triggers exist but TURSO_ENABLE_COUNTER_TRIGGERS is off (dropApito-only path would skip).
func TestDeleteRelationDocuments_hasManyHasOne_manualTriggerReferencingFKColumn(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE tenant (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE employee (id VARCHAR(36) NOT NULL PRIMARY KEY);
`).Exec(ctx)
	require.NoError(t, err)

	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})

	from := &models.ConnectionType{Model: "employee", Relation: "has_many", Type: "forward"}
	to := &models.ConnectionType{Model: "tenant", Relation: "has_one", Type: "backward"}
	require.NoError(t, drv.AddRelationFields(ctx, from, to))

	_, err = db.NewRaw(`
CREATE TRIGGER tr_manual_refs_tenant AFTER INSERT ON employee BEGIN
  SELECT CASE WHEN NEW.tenant_id IS NOT NULL THEN 1 ELSE 0 END;
END;
`).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, drv.DeleteRelationDocuments(ctx, "p1", from, to))

	var n int
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('employee') WHERE name = ?`, "tenant_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

// Mirrors Turso rosna tenant DB: mixed inline FK (role_id) + named CONSTRAINT (tenant_id); MCP showed 0 triggers.
func TestDeleteRelationDocuments_hasManyHasOne_mixedFKLikeTursoSchema(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE tenant (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE role (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE employee (
	id text(36) NOT NULL,
	tenant_id text(36), role_id VARCHAR(36) REFERENCES role (id) ON DELETE CASCADE,
	CONSTRAINT fk_employee_tenant_id_tenant_id_fk FOREIGN KEY (tenant_id) REFERENCES tenant(id) ON DELETE CASCADE
);
`).Exec(ctx)
	require.NoError(t, err)

	drv := testDriver(db, &models.DriverCredentials{Engine: _const.SQLiteDriver})
	from := &models.ConnectionType{Model: "employee", Relation: "has_many", Type: "forward"}
	to := &models.ConnectionType{Model: "tenant", Relation: "has_one", Type: "backward"}
	require.NoError(t, drv.DeleteRelationDocuments(ctx, "p1", from, to))

	var n int
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('employee') WHERE name = ?`, "tenant_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n)
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('employee') WHERE name = ?`, "role_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

// After dropping tenant_id, customer_id and location_id FOREIGN KEYs must remain (regression: naive rebuild strips all FKs).
func TestSqliteRebuildTableWithoutColumn_preservesOtherForeignKeys(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE tenant (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE customer (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE location (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE food_order (
	id VARCHAR(36) NOT NULL PRIMARY KEY,
	tenant_id VARCHAR(36),
	customer_id VARCHAR(36) REFERENCES customer(id) ON DELETE CASCADE,
	location_id VARCHAR(36) REFERENCES location(id) ON DELETE CASCADE,
	CONSTRAINT fk_food_order_tenant FOREIGN KEY (tenant_id) REFERENCES tenant(id) ON DELETE CASCADE
);
`).Exec(ctx)
	require.NoError(t, err)

	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return sqliteRebuildTableWithoutColumnTx(ctx, tx, "food_order", "tenant_id")
	})
	require.NoError(t, err)

	var n int
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('food_order') WHERE name = ?`, "tenant_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_foreign_key_list('food_order')`).Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 2, n, "customer_id and location_id FK definitions must survive rebuild")

	var fromCols []struct {
		From string `bun:"from"`
	}
	err = db.NewRaw(`SELECT "from" FROM pragma_foreign_key_list('food_order')`).Scan(ctx, &fromCols)
	require.NoError(t, err)
	for _, r := range fromCols {
		require.NotEqual(t, "tenant_id", r.From)
	}
}

// Rebuild must preserve UNIQUE INDEX on food_id AND drop only the broken FK (food→broken_food)
// while keeping the valid FK (tenant_id is dropped, so only food_id FK remains — it is invalid here
// because broken_food has no PK, so the index must still survive).
func TestSqliteRebuildTableWithoutColumn_preservesIndexes(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE tenant (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE food (id text(36));
CREATE TABLE stock (
	id text(36) NOT NULL PRIMARY KEY,
	tenant_id text(36),
	food_id text(36),
	CONSTRAINT fk_stock_tenant FOREIGN KEY (tenant_id) REFERENCES tenant(id) ON DELETE CASCADE,
	CONSTRAINT fk_stock_food FOREIGN KEY (food_id) REFERENCES food(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_stock_food_id_unique ON stock (food_id);
`).Exec(ctx)
	require.NoError(t, err)

	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return sqliteRebuildTableWithoutColumnTx(ctx, tx, "stock", "tenant_id")
	})
	require.NoError(t, err)

	var n int
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('stock') WHERE name = ?`, "tenant_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n, "tenant_id column must be gone")

	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('stock') WHERE name = ?`, "food_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n, "food_id column must survive")

	err = db.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, "idx_stock_food_id_unique").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n, "UNIQUE INDEX on food_id must survive rebuild")
}

// Reproduces remote Turso/libsql behavior where foreign_keys remains ON: ledger.id has no PK,
// so copying into a temp table with FK clauses fails. The rebuild must copy through bare columns,
// then restore surviving FK metadata and indexes.
func TestSqliteRebuildTableWithoutColumn_brokenParentPK_preservesFKWithPragmaOn(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`PRAGMA foreign_keys = OFF;`).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`
CREATE TABLE tenant (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE ledger (id text(36));
CREATE TABLE stock_draft (
	id text(36) NOT NULL PRIMARY KEY,
	tenant_id text(36),
	ledger_id text(36),
	CONSTRAINT fk_sd_tenant FOREIGN KEY (tenant_id) REFERENCES tenant(id) ON DELETE CASCADE,
	CONSTRAINT fk_sd_ledger FOREIGN KEY (ledger_id) REFERENCES ledger(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_stock_draft_ledger_id_unique ON stock_draft (ledger_id);
INSERT INTO ledger (id) VALUES ('lg1');
INSERT INTO stock_draft (id, tenant_id, ledger_id) VALUES ('s1', NULL, 'lg1');
`).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`PRAGMA foreign_keys = ON;`).Exec(ctx)
	require.NoError(t, err)

	err = sqliteRebuildTableWithoutColumnTx(ctx, db, "stock_draft", "tenant_id")
	require.NoError(t, err, "rebuild must preserve FK metadata even when parent PK metadata is damaged")

	var n int
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('stock_draft') WHERE name = ?`, "tenant_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n, "tenant_id column must be gone")

	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('stock_draft') WHERE name = ?`, "ledger_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n, "ledger_id column must survive")

	err = db.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, "idx_stock_draft_ledger_id_unique").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n, "UNIQUE INDEX on ledger_id must survive rebuild")

	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_foreign_key_list('stock_draft') WHERE "from" = ?`, "ledger_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n, "ledger_id FK metadata must survive rebuild")

	err = db.NewRaw(`SELECT COUNT(*) FROM stock_draft`).Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n, "row data must be preserved")
}

// When parent table has proper PK, the FK is valid and must be preserved through rebuild.
func TestSqliteRebuildTableWithoutColumn_preservesValidFK(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE tenant (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE customer (id VARCHAR(36) NOT NULL PRIMARY KEY);
CREATE TABLE food_order (
	id VARCHAR(36) NOT NULL PRIMARY KEY,
	tenant_id VARCHAR(36),
	customer_id VARCHAR(36),
	CONSTRAINT fk_food_order_tenant FOREIGN KEY (tenant_id) REFERENCES tenant(id) ON DELETE CASCADE,
	CONSTRAINT fk_food_order_customer FOREIGN KEY (customer_id) REFERENCES customer(id) ON DELETE CASCADE
);
CREATE UNIQUE INDEX idx_food_order_customer_id_unique ON food_order (customer_id);
`).Exec(ctx)
	require.NoError(t, err)

	err = db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return sqliteRebuildTableWithoutColumnTx(ctx, tx, "food_order", "tenant_id")
	})
	require.NoError(t, err)

	var n int
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('food_order') WHERE name = ?`, "tenant_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n, "tenant_id column must be gone")

	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_foreign_key_list('food_order')`).Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n, "customer_id FK must survive")

	err = db.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND name = ?`, "idx_food_order_customer_id_unique").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n, "UNIQUE INDEX on customer_id must survive rebuild")
}

func TestDeleteRelationDocuments_knownAsDropsOnlyTargetedFK(t *testing.T) {
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

	require.NoError(t, drv.DeleteRelationDocuments(ctx, "p1", waiterFrom, waiterTo))

	var n int
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('food_order') WHERE name = ?`, "employee_as_waiter_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 0, n)
	err = db.NewRaw(`SELECT COUNT(*) FROM pragma_table_info('food_order') WHERE name = ?`, "employee_as_chef_id").Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

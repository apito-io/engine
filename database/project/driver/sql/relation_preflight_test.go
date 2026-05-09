package sql

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestPreflightSQLiteRelationParentTables_rejectsMissingIdPK(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`CREATE TABLE role (id VARCHAR(36)); CREATE TABLE employee (id VARCHAR(36) NOT NULL PRIMARY KEY);`).Exec(ctx)
	require.NoError(t, err)

	from := &models.ConnectionType{Model: "role", Relation: "has_one"}
	to := &models.ConnectionType{Model: "employee", Relation: "has_one"}
	err = PreflightSQLiteRelationParentTablesForAddRelation(ctx, db, _const.SQLiteDriver, from, to)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "parent model role"), err.Error())
	require.True(t, strings.Contains(err.Error(), "valid primary key"), err.Error())
}

func TestPreflightSQLiteRelationParentTables_allowsNormalPK(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`CREATE TABLE role (id VARCHAR(36) NOT NULL PRIMARY KEY); CREATE TABLE employee (id VARCHAR(36) NOT NULL PRIMARY KEY);`).Exec(ctx)
	require.NoError(t, err)

	from := &models.ConnectionType{Model: "role", Relation: "has_one"}
	to := &models.ConnectionType{Model: "employee", Relation: "has_one"}
	require.NoError(t, PreflightSQLiteRelationParentTablesForAddRelation(ctx, db, _const.SQLiteDriver, from, to))
}

func TestPreflightSQLiteRelationParentTables_skipsPostgres(t *testing.T) {
	ctx := context.Background()
	from := &models.ConnectionType{Model: "role", Relation: "has_one"}
	to := &models.ConnectionType{Model: "employee", Relation: "has_one"}
	require.NoError(t, PreflightSQLiteRelationParentTablesForAddRelation(ctx, nil, _const.PostgreSQLDriver, from, to))
}

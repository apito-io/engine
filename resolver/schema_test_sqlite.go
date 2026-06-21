package resolver

import (
	stdsql "database/sql"
	"testing"

	_const "github.com/apito-io/engine/const"
	coresqlite "gitlab.com/apito.io/open_driver/project/sqlite"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func testSQLiteProjectDriver(t *testing.T) interfaces.ProjectDBInterface {
	t.Helper()
	sqldb, err := stdsql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	d := &coresqlite.Driver{}
	d.ORM = db
	d.DriverCredential = &models.DriverCredentials{Engine: _const.SQLiteDriver}
	return d
}

package sqlite

import (
	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/project/sqlcommon"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// Driver is the SQLite project SQL driver.
type Driver struct {
	sqlcommon.Driver
}

// GetDriver opens a SQLite project driver.
func GetDriver(cfg *models.Config, cred *models.DriverCredentials) (*Driver, error) {
	if cred == nil {
		return nil, sqlcommon.ErrCredentialsRequired()
	}
	ec := *cred
	ec.Engine = _const.SQLiteDriver
	base, err := sqlcommon.OpenDriver(cfg, &ec, sqliteDialect{})
	if err != nil {
		return nil, err
	}
	return &Driver{Driver: *base}, nil
}

// ApplySQLiteConnectionPragmas re-exported for pro/libsql callers.
var ApplySQLiteConnectionPragmas = sqlcommon.ApplySQLiteConnectionPragmas

// BuildPostgresDSN re-export for cross-engine callers.
var BuildPostgresDSN = sqlcommon.BuildPostgresDSN

// QuotePGIdent re-export for DDL helpers.
var QuotePGIdent = sqlcommon.QuotePGIdent

// QuoteMySQLIdent re-export for DDL helpers.
var QuoteMySQLIdent = sqlcommon.QuoteMySQLIdent

var _ interfaces.ProjectDBInterface = (*Driver)(nil)
var _ sqlcommon.BunDriver = (*Driver)(nil)
var _ sqlcommon.SQLDriverCarrier = (*Driver)(nil)

func (d *Driver) SQLDriverShell() *sqlcommon.Driver {
	if d == nil {
		return nil
	}
	return &d.Driver
}

func (d *Driver) SQLEngineName() string {
	if d == nil || d.DriverCredential == nil {
		return ""
	}
	return d.DriverCredential.Engine
}

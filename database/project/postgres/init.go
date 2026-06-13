package postgres

import (
	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/project/sqlcommon"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// Driver is the PostgreSQL project SQL driver.
type Driver struct {
	sqlcommon.Driver
}

// GetDriver opens a PostgreSQL project driver.
func GetDriver(cfg *models.Config, cred *models.DriverCredentials) (*Driver, error) {
	if cred == nil {
		return nil, sqlcommon.ErrCredentialsRequired()
	}
	ec := *cred
	ec.Engine = _const.PostgreSQLDriver
	base, err := sqlcommon.OpenDriver(cfg, &ec, postgresDialect{})
	if err != nil {
		return nil, err
	}
	return &Driver{Driver: *base}, nil
}

var BuildPostgresDSN = sqlcommon.BuildPostgresDSN
var QuotePGIdent = sqlcommon.QuotePGIdent
var QuoteMySQLIdent = sqlcommon.QuoteMySQLIdent

var _ interfaces.ProjectDBInterface = (*Driver)(nil)
var _ sqlcommon.BunDriver = (*Driver)(nil)

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

var _ sqlcommon.SQLDriverCarrier = (*Driver)(nil)

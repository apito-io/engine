package postgres

import (
	"github.com/apito-io/engine/database/project/sqlcommon"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

func openBootstrapBun(cfg *models.Config, cred *models.DriverCredentials) (*Driver, error) {
	bunDB, err := sqlcommon.OpenBun(cfg, cred)
	if err != nil {
		return nil, err
	}
	return &Driver{
		Driver: sqlcommon.Driver{
			Base: sqlcommon.Base{
				Conf:             cfg,
				ORM:              bunDB,
				DriverCredential: cred,
				Dialect:          postgresDialect{},
			},
		},
	}, nil
}

func (d *Driver) adoptOpenedDriver(db *Driver) {
	if db == nil {
		return
	}
	d.ORM = db.ORM
	d.DriverCredential = db.DriverCredential
}

func (d *Driver) adoptOpenedBun(orm *bun.DB, cred *models.DriverCredentials) {
	if orm == nil || cred == nil {
		return
	}
	d.ORM = orm
	d.DriverCredential = cred
}

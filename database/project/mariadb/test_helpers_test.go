package mariadb

import (
	"github.com/apito-io/engine/database/project/sqlcommon"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

func testDriver(orm *bun.DB, cred *models.DriverCredentials) *Driver {
	return &Driver{Driver: sqlcommon.Driver{Base: sqlcommon.Base{ORM: orm, DriverCredential: cred}}}
}

func testDriverWithConf(orm *bun.DB, cfg *models.Config, cred *models.DriverCredentials) *Driver {
	return &Driver{Driver: sqlcommon.Driver{Base: sqlcommon.Base{ORM: orm, Conf: cfg, DriverCredential: cred}}}
}

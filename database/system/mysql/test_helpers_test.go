package mysql

import (
	"github.com/apito-io/engine/database/system/sqlcommon"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

func testSystemDriver(orm *bun.DB, cfg *models.Config, cred *models.DriverCredentials) *Driver {
	return &Driver{Base: sqlcommon.Base{ORM: orm, Conf: cfg, DriverCredential: cred}}
}

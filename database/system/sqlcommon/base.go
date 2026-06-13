package sqlcommon

import (
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

// Base holds shared Bun ORM state for all SQL system drivers.
type Base struct {
	Conf             *models.Config
	ORM              *bun.DB
	DriverCredential *models.DriverCredentials
}

// Driver embeds Base for system SQL drivers.
type Driver struct {
	Base
}

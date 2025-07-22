package system

import (
	"context"
	"errors"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/system/driver/badger"
	"github.com/apito-io/engine/database/system/driver/sql"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

func GetSystemDriver(engineConfig *models.DriverCredentials, conf *models.Config) (interfaces.ApitoSystemDB, error) {

	if engineConfig == nil {
		return nil, errors.New("please select a system database engine")
	}

	// Check if pro factory is injected (dependency injection for pro version)
	if conf != nil && conf.DriverFactory != nil {
		if factory, ok := conf.DriverFactory.(interfaces.DatabaseDriverFactory); ok {
			if factory.SupportsEngine(engineConfig.Engine) {
				return factory.CreateSystemDriver(conf, engineConfig)
			}
		}
	}

	// Fall back to default core drivers
	ctx := context.Background()

	var db interfaces.ApitoSystemDB
	var err error

	switch engineConfig.Engine {
	case _const.PostgreSQLDriver, _const.MySQLDriver, _const.MariaDBDriver:
		db, err = sql.GetSystemSQLDriver(engineConfig)
		if err != nil {
			return nil, err
		}
		err = db.RunMigration(ctx)
	case _const.EmbeddedDB:
		db, err = badger.GetSystemBadgerDriver(engineConfig)
	default: // default set embedded database
		db, err = badger.GetSystemBadgerDriver(engineConfig)
	}
	if err != nil {
		return nil, err
	}
	return db, nil
}

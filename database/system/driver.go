package system

import (
	"errors"
	"log"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/system/driver/bbolt"
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

	var db interfaces.ApitoSystemDB
	var err error

	log.Printf("Getting system driver: %s", engineConfig.Engine)

	switch engineConfig.Engine {
	case _const.CoreDB:
		db, err = bbolt.GetSystemBBoltDriver(engineConfig, nil)
	default: // default set embedded database
		db, err = bbolt.GetSystemBBoltDriver(engineConfig, nil)
	}
	if err != nil {
		return nil, err
	}
	return db, nil
}

package system

import (
	"errors"
	"log"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/system/bbolt"
	mongodb "github.com/apito-io/engine/database/system/mongodb"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

func GetSystemDriver(conf *models.Config, engineConfig *models.DriverCredentials) (interfaces.ApitoSystemDB, error) {

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
	case _const.CoreDB, "coreDB":
		db, err = bbolt.GetSystemBBoltDriver(conf, engineConfig, nil)
	case _const.MongoDBDriver:
		db, err = mongodb.GetSystemMongoDriver(engineConfig, conf)
	default: // default set embedded database
		db, err = bbolt.GetSystemBBoltDriver(conf, engineConfig, nil)
	}
	if err != nil {
		return nil, err
	}
	return db, nil
}

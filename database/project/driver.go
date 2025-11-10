package project

import (
	"errors"
	"log"

	_const "github.com/apito-io/engine/const"
	//"github.com/apito-io/engine/database/project/driver/mongo"
	"github.com/apito-io/engine/database/project/driver/bbolt"
	"github.com/apito-io/engine/database/project/driver/sql"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

func GetProjectDriver(engineConfig *models.DriverCredentials, cfg *models.Config) (interfaces.ProjectDBInterface, error) {
	return GetProjectDriverWithConfig(cfg, engineConfig)
}

func GetProjectDriverWithConfig(conf *models.Config, engineConfig *models.DriverCredentials) (interfaces.ProjectDBInterface, error) {
	// Check if pro factory is injected (dependency injection for pro version)
	if conf != nil && conf.DriverFactory != nil {
		if factory, ok := conf.DriverFactory.(interfaces.DatabaseDriverFactory); ok {
			if factory.SupportsEngine(engineConfig.Engine) {
				return factory.CreateProjectDriver(conf, engineConfig)
			}
		}
	}

	log.Printf("Getting project driver: %s", engineConfig.Engine)

	// Fall back to default core drivers
	var db interfaces.ProjectDBInterface
	var err error
	switch engineConfig.Engine {
	case _const.CoreDB:
		db, err = bbolt.GetBBoltDriver(conf, engineConfig)
	case //_const.PostgreSQLDriver,
		//_const.MySQLDriver,
		//_const.MariaDBDriver,
		_const.SQLiteDriver:
		db, err = sql.GetSQLDriver(conf, engineConfig)
	//case _const.MongoDBDriver:
	//	db, err = mongo.GetMongoDriver(engineConfig)
	default:
		return nil, errors.New("unsupported database driver passed")
	}
	if err != nil {
		return nil, err
	}
	return db, nil
}

package project

import (
	"errors"
	"log"

	_const "github.com/apito-io/engine/const"
	//"github.com/apito-io/engine/database/project/mongo"
	"github.com/apito-io/engine/database/project/bbolt"
	"github.com/apito-io/engine/database/project/mongo"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/telemetry"
)

func GetProjectDriver(engineConfig *models.DriverCredentials, cfg *models.Config) (interfaces.ProjectDBInterface, error) {
	return GetProjectDriverWithConfig(cfg, engineConfig, nil)
}

func GetProjectDriverWithConfig(conf *models.Config, engineConfig *models.DriverCredentials, proExtras interface{}) (interfaces.ProjectDBInterface, error) {
	// Check if pro factory is injected (dependency injection for pro version)
	if conf != nil && conf.DriverFactory != nil {
		if pf, ok := conf.DriverFactory.(interfaces.ProProjectDriverFactory); ok {
			if pf.SupportsEngine(engineConfig.Engine) {
				// Do not wrap here: callers may type-assert to ProProjectDBInterface on the concrete driver.
				return pf.CreateProjectDriverWithProExtras(conf, engineConfig, proExtras)
			}
		}
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
	case _const.CoreDB, "coreDB":
		db, err = bbolt.GetBBoltDriver(conf, engineConfig)
	case _const.SQLiteDriver, _const.PostgreSQLDriver, _const.MySQLDriver, _const.MariaDBDriver:
		db, err = GetProjectSQLDriver(conf, engineConfig)
	case _const.MongoDBDriver:
		db, err = mongo.GetProjectMongoDriver(conf, engineConfig)
	default:
		return nil, errors.New("unsupported database driver passed")
	}
	if err != nil {
		return nil, err
	}
	db = telemetry.WrapProjectDBWithMetrics(conf, engineConfig.Engine, db)
	return db, nil
}

package shared

import (
	"errors"

	_const "github.com/apito-io/engine/const"
	sharedRedis "github.com/apito-io/engine/database/shared/redis"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

func GetSharedDriver(engineConfig *models.DriverCredentials) (interfaces.SharedDBInterface, error) {
	var db interfaces.SharedDBInterface
	var err error
	if engineConfig == nil { // default db
		return sharedRedis.GetSharedRedisDriver(engineConfig)
	}
	switch engineConfig.Engine {
	case _const.RedisDriver:
		db, err = sharedRedis.GetSharedRedisDriver(engineConfig)
		break
	default:
		return nil, errors.New("unsupported database driver passed")
	}
	if err != nil {
		return nil, err
	}
	return db, nil
}

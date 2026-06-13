package system

import (
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/system/mariadb"
	"github.com/apito-io/engine/database/system/mysql"
	"github.com/apito-io/engine/database/system/postgres"
	"github.com/apito-io/engine/database/system/sqlite"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// GetSQLSystemDriver routes to the per-engine open-core system SQL driver.
func GetSQLSystemDriver(cfg *models.Config, cred *models.DriverCredentials) (interfaces.ApitoSystemDB, error) {
	if cred == nil {
		return nil, fmt.Errorf("driver credentials are required")
	}
	engine := strings.ToLower(strings.TrimSpace(cred.Engine))
	switch engine {
	case strings.ToLower(_const.SQLiteDriver):
		return sqlite.GetSQLiteSystemDriver(cfg, cred)
	case strings.ToLower(_const.PostgreSQLDriver):
		return postgres.GetPostgresSystemDriver(cfg, cred)
	case strings.ToLower(_const.MySQLDriver):
		return mysql.GetMySQLSystemDriver(cfg, cred)
	case strings.ToLower(_const.MariaDBDriver):
		return mariadb.GetMariaDBSystemDriver(cfg, cred)
	default:
		return nil, fmt.Errorf("unsupported SQL system engine: %s", cred.Engine)
	}
}

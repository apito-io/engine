package project

import (
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/project/mariadb"
	"github.com/apito-io/engine/database/project/mysql"
	"github.com/apito-io/engine/database/project/postgres"
	"github.com/apito-io/engine/database/project/sqlite"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// GetProjectSQLDriver routes to the per-engine project driver package.
func GetProjectSQLDriver(cfg *models.Config, cred *models.DriverCredentials) (interfaces.ProjectDBInterface, error) {
	if cred == nil {
		return nil, fmt.Errorf("driver credentials are required")
	}
	engine := strings.ToLower(strings.TrimSpace(cred.Engine))
	switch engine {
	case strings.ToLower(_const.SQLiteDriver):
		return sqlite.GetDriver(cfg, cred)
	case strings.ToLower(_const.PostgreSQLDriver):
		return postgres.GetDriver(cfg, cred)
	case strings.ToLower(_const.MySQLDriver):
		return mysql.GetDriver(cfg, cred)
	case strings.ToLower(_const.MariaDBDriver):
		return mariadb.GetDriver(cfg, cred)
	default:
		return nil, fmt.Errorf("unsupported SQL engine: %s", cred.Engine)
	}
}

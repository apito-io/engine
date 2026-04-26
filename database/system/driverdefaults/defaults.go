// Package driverdefaults holds shared defaults for system DB drivers (SQL, Mongo, etc.).
package driverdefaults

import (
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/database/system/bootstrapmeta"
	"github.com/apito-io/engine/models"
)

// OSSBootstrapProjectDriver returns fixed starter-project credentials for first boot when the
// system database engine matches a supported OSS layout. There is no global PROJECT_DB_* template.
func OSSBootstrapProjectDriver(cfg *models.Config, projectID string) *models.DriverCredentials {
	if cfg == nil || projectID != bootstrapmeta.StarterProjectID {
		return nil
	}
	eng := strings.ToLower(strings.TrimSpace(cfg.SystemDatabaseEngine))
	switch eng {
	case "", strings.ToLower(_const.CoreDB):
		return &models.DriverCredentials{
			ProjectID: projectID,
			Engine:    _const.CoreDB,
			File:      bootstrapmeta.OSSStarterSQLiteFile,
		}
	case strings.ToLower(_const.MongoDBDriver):
		if strings.TrimSpace(cfg.SystemDBHost) == "" {
			return nil
		}
		return &models.DriverCredentials{
			ProjectID: projectID,
			Engine:    _const.MongoDBDriver,
			Host:      cfg.SystemDBHost,
			Port:      cfg.SystemDBPort,
			User:      cfg.SystemDBUser,
			Password:  cfg.SystemDBPassword,
			Database:  bootstrapmeta.StarterMongoDatabaseName,
		}
	case strings.ToLower(_const.PostgreSQLDriver), "postgres":
		if strings.TrimSpace(cfg.SystemDBHost) == "" {
			return nil
		}
		return &models.DriverCredentials{
			ProjectID: projectID,
			Engine:    _const.PostgreSQLDriver,
			Host:      cfg.SystemDBHost,
			Port:      cfg.SystemDBPort,
			User:      cfg.SystemDBUser,
			Password:  cfg.SystemDBPassword,
			Database:  bootstrapmeta.StarterPostgresDatabaseName,
		}
	default:
		return nil
	}
}

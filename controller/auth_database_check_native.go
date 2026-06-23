//go:build !cloudflare

package controller

import (
	"net/http"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"gitlab.com/apito.io/open_driver/project"
	"gitlab.com/apito.io/open_driver/project/bbolt"
	"gitlab.com/apito.io/open_driver/project/mongo"
	"github.com/labstack/echo/v4"
)

// DatabaseCheckCore runs open-core database connectivity checks (Mongo, SQL family, coredb).
func DatabaseCheckCore(a *AuthController, c echo.Context, req *DatabaseRequest) error {
	dbType := strings.ToLower(strings.TrimSpace(req.Type))

	switch dbType {
	case _const.MongoDBDriver:
		driver, err := mongo.GetProjectMongoDriver(a.Cfg, &models.DriverCredentials{
			Engine: _const.MongoDBDriver, Host: req.Host, Port: DatabaseCheckPort(req.Port, "27017"),
			Database: req.Database, User: req.Username, Password: req.Password,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{Message: err.Error(), Code: http.StatusInternalServerError})
		}
		if err = driver.Ping(); err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{Message: err.Error(), Code: http.StatusInternalServerError})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{"code": http.StatusOK, "message": "Database connected successfully"})

	case _const.PostgreSQLDriver, _const.MySQLDriver, _const.SQLiteDriver, _const.SQLServerDriver, _const.MariaDBDriver:
		defPort := ""
		switch dbType {
		case _const.PostgreSQLDriver:
			defPort = "5432"
		case _const.MySQLDriver, _const.MariaDBDriver:
			defPort = "3306"
		}
		driver, err := project.GetProjectSQLDriver(a.Cfg, &models.DriverCredentials{
			File: req.File, Engine: dbType, Host: req.Host, Port: DatabaseCheckPort(req.Port, defPort),
			Database: req.Database, User: req.Username, Password: req.Password, SSLMode: req.SSLMode,
		})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{Message: err.Error(), Code: http.StatusInternalServerError})
		}
		pinger, ok := driver.(interface{ Ping() error })
		if !ok {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{Message: "driver does not support connectivity check", Code: http.StatusInternalServerError})
		}
		if err = pinger.Ping(); err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{Message: err.Error(), Code: http.StatusInternalServerError})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{"code": http.StatusOK, "message": "Database connected successfully"})

	case _const.CoreDB:
		driver, err := bbolt.GetBBoltDriver(a.Cfg, &models.DriverCredentials{Engine: _const.CoreDB, File: req.File})
		if err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{Message: err.Error(), Code: http.StatusInternalServerError})
		}
		if err = driver.Ping(); err != nil {
			return c.JSON(http.StatusInternalServerError, &models.HttpResponse{Message: err.Error(), Code: http.StatusInternalServerError})
		}
		return c.JSON(http.StatusOK, map[string]interface{}{"code": http.StatusOK, "message": "Database connected successfully"})
	default:
		return c.JSON(http.StatusBadRequest, &models.HttpResponse{Message: "Invalid database type", Code: http.StatusBadRequest})
	}
}

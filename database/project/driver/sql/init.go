package sql

import (
	"fmt"
	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type SQLDriver struct {
	Gorm             *gorm.DB
	DriverCredential *models.DriverCredentials
}

func GetSQLDriver(driverCredentials *models.DriverCredentials) (*SQLDriver, error) {

	var gormDB *gorm.DB
	var err error

	switch driverCredentials.Engine {
	case _const.MySQLDriver, _const.MariaDBDriver:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			driverCredentials.User, driverCredentials.Password, driverCredentials.Host, driverCredentials.Port, driverCredentials.Database)
		gormDB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	case _const.PostgreSQLDriver:
		dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s",
			driverCredentials.Host, driverCredentials.Port, driverCredentials.User, driverCredentials.Password, driverCredentials.Database)
		gormDB, err = gorm.Open(postgres.New(postgres.Config{
			DSN: dsn,
		}), &gorm.Config{})
	}

	if err != nil {
		return nil, err
	}

	return &SQLDriver{Gorm: gormDB, DriverCredential: driverCredentials}, nil
}

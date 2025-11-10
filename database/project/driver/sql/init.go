package sql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	_ "github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
)

type SQLDriver struct {
	Conf             *models.Config
	ORM              *bun.DB
	DriverCredential *models.DriverCredentials
}

func GetSQLDriver(cfg *models.Config, driverCredentials *models.DriverCredentials) (*SQLDriver, error) {
	var sqlDB *sql.DB
	var err error
	var bunDB *bun.DB

	switch driverCredentials.Engine {
	case _const.SQLiteDriver:
		dbPath := filepath.Join(cfg.DefaultDatabaseDir, driverCredentials.File)
		if !strings.HasSuffix(dbPath, ".sqlite") {
			return nil, fmt.Errorf("database file must end with .sqlite")
		}

		dbPath, err = utility.ExpandPath(dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to expand database path %s: %v", dbPath, err)
		}

		sqldb, err := sql.Open(sqliteshim.ShimName, fmt.Sprintf("file:%s?cache=shared&mode=rwc", dbPath))
		if err != nil {
			return nil, err
		}
		bunDB = bun.NewDB(sqldb, sqlitedialect.New())

	case _const.MySQLDriver, _const.MariaDBDriver:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			driverCredentials.User, driverCredentials.Password, driverCredentials.Host, driverCredentials.Port, driverCredentials.Database)
		sqlDB, err = sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		bunDB = bun.NewDB(sqlDB, mysqldialect.New())
	case _const.PostgreSQLDriver:
		dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			driverCredentials.User, driverCredentials.Password, driverCredentials.Host, driverCredentials.Port, driverCredentials.Database)
		sqlDB = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		bunDB = bun.NewDB(sqlDB, pgdialect.New())
	default:
		return nil, fmt.Errorf("unsupported database engine: %s", driverCredentials.Engine)
	}

	log.Printf("SQL database connected successfully: %s", driverCredentials.File)

	return &SQLDriver{Conf: cfg, ORM: bunDB, DriverCredential: driverCredentials}, nil
}

func (p *SQLDriver) Ping() error {
	ctx := context.Background()

	// Test connection with a simple query
	var result int
	err := p.ORM.NewSelect().ColumnExpr("1").Scan(ctx, &result)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	return nil
}

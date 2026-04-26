package sql

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/url"
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

		dsn := fmt.Sprintf("file:%s?cache=shared&mode=rwc", dbPath)
		sqldb, err := sql.Open(sqliteshim.ShimName, dsn)
		if err != nil {
			return nil, err
		}
		bunDB = bun.NewDB(sqldb, sqlitedialect.New())
		if err := ApplySQLiteConnectionPragmas(context.Background(), bunDB, driverCredentials.Engine, dsn); err != nil {
			_ = bunDB.Close()
			return nil, err
		}

	case _const.MySQLDriver, _const.MariaDBDriver:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			driverCredentials.User, driverCredentials.Password, driverCredentials.Host, driverCredentials.Port, driverCredentials.Database)
		sqlDB, err = sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		bunDB = bun.NewDB(sqlDB, mysqldialect.New())
	case _const.PostgreSQLDriver:
		dsn, err := BuildPostgresDSN(driverCredentials)
		if err != nil {
			return nil, err
		}
		sqlDB = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		bunDB = bun.NewDB(sqlDB, pgdialect.New())

	default:
		return nil, fmt.Errorf("unsupported database engine: %s", driverCredentials.Engine)
	}

	log.Printf("SQL database connected successfully: engine=%s file=%s db=%s", driverCredentials.Engine, driverCredentials.File, driverCredentials.Database)

	return &SQLDriver{Conf: cfg, ORM: bunDB, DriverCredential: driverCredentials}, nil
}

// BuildPostgresDSN builds a libpq URL for pgdriver from credentials (sslmode from SSLMode, default disable).
func BuildPostgresDSN(c *models.DriverCredentials) (string, error) {
	if c.Host == "" || c.Database == "" {
		return "", fmt.Errorf("postgres: host and database are required")
	}
	port := c.Port
	if port == "" {
		port = "5432"
	}
	ssl := c.SSLMode
	if ssl == "" {
		ssl = "disable"
	}
	hostPort := net.JoinHostPort(c.Host, port)
	u := &url.URL{
		Scheme: "postgres",
		Host:   hostPort,
		Path:   "/" + strings.TrimPrefix(strings.TrimSpace(c.Database), "/"),
	}
	if c.User != "" || c.Password != "" {
		u.User = url.UserPassword(c.User, c.Password)
	}
	q := u.Query()
	q.Set("sslmode", ssl)
	if sch := strings.TrimSpace(c.Schema); sch != "" {
		// Per-project schema isolation (GENERAL_POSTGRES_ISOLATION=schema): pin search_path for pooled connections.
		q.Set("options", "-csearch_path="+sch+",public")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Close implements database.Closeable so ConnectionManager can release *sql.DB on eviction.
func (p *SQLDriver) Close() error {
	if p == nil || p.ORM == nil {
		return nil
	}
	return p.ORM.Close()
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

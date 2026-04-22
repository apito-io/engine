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
	_ "github.com/tursodatabase/libsql-client-go/libsql"
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
		dsn, err := BuildPostgresDSN(driverCredentials)
		if err != nil {
			return nil, err
		}
		sqlDB = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		bunDB = bun.NewDB(sqlDB, pgdialect.New())

	case "libsql":
		dsn, err := buildLibSQLDSN(driverCredentials)
		if err != nil {
			return nil, err
		}
		sqldb, err := sql.Open("libsql", dsn)
		if err != nil {
			return nil, err
		}
		bunDB = bun.NewDB(sqldb, sqlitedialect.New())

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
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// buildLibSQLDSN supports full URLs in Database (libsql://, https://, file:) or Turso host form:
// Database = db name, Host = organization slug, Password = database auth token.
func buildLibSQLDSN(c *models.DriverCredentials) (string, error) {
	if c.Database != "" && (strings.HasPrefix(c.Database, "libsql://") ||
		strings.HasPrefix(c.Database, "http://") ||
		strings.HasPrefix(c.Database, "https://") ||
		strings.HasPrefix(c.Database, "file:")) {
		if strings.TrimSpace(c.Password) == "" {
			return c.Database, nil
		}
		u, err := url.Parse(c.Database)
		if err != nil {
			return c.Database, nil
		}
		q := u.Query()
		if q.Get("authToken") == "" {
			q.Set("authToken", c.Password)
			u.RawQuery = q.Encode()
		}
		return u.String(), nil
	}
	token := c.Password
	if c.Host == "" || c.Database == "" {
		return "", fmt.Errorf("libsql: Host (organization) and Database (db name) are required when not using a full URL")
	}
	u := url.URL{
		Scheme: "libsql",
		Host:   fmt.Sprintf("%s-%s.turso.io", c.Database, c.Host),
	}
	q := u.Query()
	q.Set("authToken", token)
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

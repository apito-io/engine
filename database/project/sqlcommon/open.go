package sqlcommon

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"net/url"
	"path/filepath"
	"strings"
	"time"

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

// OpenDriver opens a Bun handle and returns a Driver with the given dialect.
func OpenDriver(cfg *models.Config, cred *models.DriverCredentials, dialect Dialect) (*Driver, error) {
	if cfg == nil || cred == nil {
		return nil, fmt.Errorf("sqlcommon: config and credentials are required")
	}
	if dialect == nil {
		return nil, fmt.Errorf("sqlcommon: dialect is required")
	}

	bunDB, err := openBun(cfg, cred)
	if err != nil {
		return nil, err
	}

	log.Printf("SQL database connected successfully: engine=%s file=%s db=%s", cred.Engine, cred.File, cred.Database)

	return &Driver{
		Base: Base{
			Conf:             cfg,
			ORM:              bunDB,
			DriverCredential: cred,
			Dialect:          dialect,
		},
	}, nil
}

// OpenBun opens a Bun handle for the given credentials (any supported SQL engine).
func OpenBun(cfg *models.Config, cred *models.DriverCredentials) (*bun.DB, error) {
	return openBun(cfg, cred)
}

func openBun(cfg *models.Config, cred *models.DriverCredentials) (*bun.DB, error) {
	var sqlDB *sql.DB
	var err error
	var bunDB *bun.DB

	switch cred.Engine {
	case _const.SQLiteDriver:
		dbPath := filepath.Join(cfg.DefaultDatabaseDir, cred.File)
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
		sqldb.SetMaxOpenConns(1)
		sqldb.SetMaxIdleConns(1)
		sqldb.SetConnMaxLifetime(10 * time.Minute)
		sqldb.SetConnMaxIdleTime(5 * time.Minute)
		bunDB = bun.NewDB(sqldb, sqlitedialect.New())
		if err := ApplySQLiteConnectionPragmas(context.Background(), bunDB, cred.Engine, dsn); err != nil {
			_ = bunDB.Close()
			return nil, err
		}
	case _const.MySQLDriver, _const.MariaDBDriver:
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			cred.User, cred.Password, cred.Host, cred.Port, cred.Database)
		sqlDB, err = sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		sqlDB.SetConnMaxLifetime(10 * time.Minute)
		sqlDB.SetConnMaxIdleTime(5 * time.Minute)
		bunDB = bun.NewDB(sqlDB, mysqldialect.New())
	case _const.PostgreSQLDriver:
		dsn, err := BuildPostgresDSN(cred)
		if err != nil {
			return nil, err
		}
		sqlDB = sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		sqlDB.SetConnMaxLifetime(10 * time.Minute)
		sqlDB.SetConnMaxIdleTime(5 * time.Minute)
		bunDB = bun.NewDB(sqlDB, pgdialect.New())
	default:
		return nil, fmt.Errorf("unsupported database engine: %s", cred.Engine)
	}
	return bunDB, nil
}

// BuildPostgresDSN builds a libpq URL for pgdriver from credentials.
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
		q.Set("options", "-csearch_path="+sch+",public")
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// Close releases the underlying Bun pool.
func (d *Driver) Close() error {
	if d == nil || d.Base.ORM == nil {
		return nil
	}
	return d.Base.ORM.Close()
}

// Ping verifies the connection is alive.
func (d *Driver) Ping() error {
	ctx := context.Background()
	var result int
	err := d.Base.ORM.NewSelect().ColumnExpr("1").Scan(ctx, &result)
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	return nil
}

// Bun returns the Bun handle (for dbexplorer and pro wrappers).
func (d *Driver) Bun() *bun.DB {
	if d == nil {
		return nil
	}
	return d.Base.ORM
}

// BunDriver is implemented by all SQL project drivers for introspection tools.
type BunDriver interface {
	Bun() *bun.DB
	Engine() string
}

// SQLDriverCarrier is implemented by open-core and pro SQL project drivers for dbexplorer unwrap.
type SQLDriverCarrier interface {
	SQLDriverShell() *Driver
	SQLEngineName() string
}

var _ BunDriver = (*Driver)(nil)

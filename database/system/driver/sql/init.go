package sql

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	_ "github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
)

// SystemSQLDriver is the open-core system DB implementation for PostgreSQL, MySQL/MariaDB, and SQLite.
type SystemSQLDriver struct {
	Conf             *models.Config
	ORM              *bun.DB
	DriverCredential *models.DriverCredentials
}

func buildPostgresSystemDSN(c *models.DriverCredentials) (string, error) {
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

// GetSystemSQLDriver opens a Bun-backed system database for PostgreSQL, MySQL, MariaDB, or SQLite only.
func GetSystemSQLDriver(cfg *models.Config, driverCredentials *models.DriverCredentials) (*SystemSQLDriver, error) {
	if driverCredentials == nil {
		return nil, fmt.Errorf("driver credentials are required")
	}

	engine := strings.ToLower(strings.TrimSpace(driverCredentials.Engine))
	baseDir := "."
	if cfg != nil && cfg.DefaultDatabaseDir != "" {
		baseDir = cfg.DefaultDatabaseDir
	}

	var orm *bun.DB
	var err error

	switch engine {
	case strings.ToLower(_const.MySQLDriver), strings.ToLower(_const.MariaDBDriver):
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			driverCredentials.User, driverCredentials.Password,
			driverCredentials.Host, driverCredentials.Port, driverCredentials.Database)
		sqldb, err := sql.Open("mysql", dsn)
		if err != nil {
			return nil, err
		}
		orm = bun.NewDB(sqldb, mysqldialect.New())

	case strings.ToLower(_const.SQLiteDriver):
		fileName := driverCredentials.File
		if fileName == "" {
			if driverCredentials.Database == "" {
				return nil, fmt.Errorf("sqlite: database file or database name is required")
			}
			fileName = driverCredentials.Database + ".sqlite"
		}
		if !strings.HasSuffix(fileName, ".sqlite") {
			return nil, fmt.Errorf("database file must end with .sqlite")
		}
		dbPath := filepath.Join(baseDir, fileName)
		dbPath, err = utility.ExpandPath(dbPath)
		if err != nil {
			return nil, fmt.Errorf("failed to expand database path %s: %v", dbPath, err)
		}
		sqldb, err := sql.Open(sqliteshim.ShimName, fmt.Sprintf("file:%s?cache=shared&mode=rwc", dbPath))
		if err != nil {
			return nil, err
		}
		orm = bun.NewDB(sqldb, sqlitedialect.New())

	case strings.ToLower(_const.PostgreSQLDriver):
		dsn, err := buildPostgresSystemDSN(driverCredentials)
		if err != nil {
			return nil, err
		}
		sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
		orm = bun.NewDB(sqldb, pgdialect.New())
		if err := orm.Ping(); err != nil {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("unsupported system database engine: %s", driverCredentials.Engine)
	}

	RegisterSystemSQLSchemaModels(orm)

	return &SystemSQLDriver{
		Conf:             cfg,
		ORM:              orm,
		DriverCredential: driverCredentials,
	}, nil
}

// Ping verifies the database connection.
func (p *SystemSQLDriver) Ping() error {
	ctx := context.Background()
	if p == nil || p.ORM == nil {
		return fmt.Errorf("system sql driver not initialized")
	}
	switch p.ORM.Dialect().Name() {
	case dialect.SQLite, dialect.PG, dialect.MySQL:
		var result int
		return p.ORM.NewSelect().ColumnExpr("1").Scan(ctx, &result)
	default:
		return fmt.Errorf("database dialect not supported: %s", p.ORM.Dialect().Name())
	}
}

// Close releases the underlying *sql.DB.
func (p *SystemSQLDriver) Close() error {
	if p == nil || p.ORM == nil {
		return nil
	}
	return p.ORM.Close()
}

// RunMigration creates core system tables if they do not exist.
func (p *SystemSQLDriver) RunMigration(ctx context.Context) error {
	RegisterSystemSQLSchemaModels(p.ORM)
	for _, m := range systemSQLSchemaModels() {
		_, err := p.ORM.NewCreateTable().IfNotExists().Model(m).Exec(ctx)
		if err != nil {
			return err
		}
	}
	return p.ensureSystemSecondaryIndexes(ctx)
}

// EnsureSystemBootstrap creates idempotent first-run data (default admin, org/team/project) like Mongo/Arango.
func (p *SystemSQLDriver) EnsureSystemBootstrap(ctx context.Context) error {
	if err := p.ensureBootstrapAdmin(ctx); err != nil {
		return err
	}
	if err := p.ensureBootstrapOrgTeamProject(ctx); err != nil {
		return err
	}
	return p.ensureStarterProjectDriver(ctx)
}

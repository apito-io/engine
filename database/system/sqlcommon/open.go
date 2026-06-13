package sqlcommon

import (
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
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/driver/sqliteshim"
)

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
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// OpenSQLite opens a local SQLite system database.
func OpenSQLite(cfg *models.Config, cred *models.DriverCredentials) (*bun.DB, error) {
	baseDir := "."
	if cfg != nil && cfg.DefaultDatabaseDir != "" {
		baseDir = cfg.DefaultDatabaseDir
	}
	fileName := cred.File
	if fileName == "" {
		if cred.Database == "" {
			return nil, fmt.Errorf("sqlite: database file or database name is required")
		}
		fileName = cred.Database + ".sqlite"
	}
	if !strings.HasSuffix(fileName, ".sqlite") {
		return nil, fmt.Errorf("database file must end with .sqlite")
	}
	dbPath := filepath.Join(baseDir, fileName)
	var err error
	dbPath, err = utility.ExpandPath(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to expand database path %s: %v", dbPath, err)
	}
	sqldb, err := sql.Open(sqliteshim.ShimName, fmt.Sprintf("file:%s?cache=shared&mode=rwc", dbPath))
	if err != nil {
		return nil, err
	}
	return bun.NewDB(sqldb, sqlitedialect.New()), nil
}

// OpenPostgres opens a PostgreSQL system database.
func OpenPostgres(cred *models.DriverCredentials) (*bun.DB, error) {
	dsn, err := BuildPostgresDSN(cred)
	if err != nil {
		return nil, err
	}
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	orm := bun.NewDB(sqldb, pgdialect.New())
	if err := orm.Ping(); err != nil {
		return nil, err
	}
	return orm, nil
}

// OpenMySQL opens a MySQL system database.
func OpenMySQL(cred *models.DriverCredentials) (*bun.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cred.User, cred.Password, cred.Host, cred.Port, cred.Database)
	sqldb, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	return bun.NewDB(sqldb, mysqldialect.New()), nil
}

// OpenMariaDB opens a MariaDB system database (same driver as MySQL).
func OpenMariaDB(cred *models.DriverCredentials) (*bun.DB, error) {
	return OpenMySQL(cred)
}

// NormalizeEngine returns the canonical engine constant.
func NormalizeEngine(engine string) string {
	return strings.ToLower(strings.TrimSpace(engine))
}

// ErrCredentialsRequired is returned when driver credentials are missing.
func ErrCredentialsRequired() error {
	return fmt.Errorf("driver credentials are required")
}

// SupportedEngine reports whether the engine string is a supported open-core system SQL engine.
func SupportedEngine(engine string) bool {
	switch NormalizeEngine(engine) {
	case strings.ToLower(_const.SQLiteDriver),
		strings.ToLower(_const.PostgreSQLDriver),
		strings.ToLower(_const.MySQLDriver),
		strings.ToLower(_const.MariaDBDriver):
		return true
	default:
		return false
	}
}

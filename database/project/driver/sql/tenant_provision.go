package sql

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/go-sql-driver/mysql"
	"github.com/uptrace/bun/driver/pgdriver"
)

// CreatePostgresDatabaseIfNotExists runs CREATE DATABASE against the admin "postgres" database.
func CreatePostgresDatabaseIfNotExists(ctx context.Context, base *models.DriverCredentials, dbName string) error {
	if dbName == "" {
		return fmt.Errorf("postgres: empty database name")
	}
	admin := *base
	admin.Database = "postgres"
	dsn, err := BuildPostgresDSN(&admin)
	if err != nil {
		return err
	}
	db := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))
	defer db.Close()

	safe := strings.ReplaceAll(dbName, `"`, `""`)
	stmt := fmt.Sprintf(`CREATE DATABASE "%s"`, safe)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return nil
		}
		return err
	}
	return nil
}

// CreateMySQLDatabaseIfNotExists runs CREATE DATABASE IF NOT EXISTS against the "mysql" system database.
func CreateMySQLDatabaseIfNotExists(ctx context.Context, base *models.DriverCredentials, dbName string) error {
	if dbName == "" {
		return fmt.Errorf("mysql: empty database name")
	}
	cfg := mysql.Config{
		User:                 base.User,
		Passwd:               base.Password,
		Net:                  "tcp",
		Addr:                 base.Host + ":" + base.Port,
		DBName:               "mysql",
		AllowNativePasswords: true,
	}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()

	safe := "`" + strings.ReplaceAll(dbName, "`", "``") + "`"
	if _, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+safe); err != nil {
		return err
	}
	return nil
}


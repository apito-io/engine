package sql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

func pgQuoteIdent(s string) string {
	s = strings.ReplaceAll(s, `"`, `""`)
	return `"` + s + `"`
}

// QuotePGIdent returns a PostgreSQL double-quoted identifier for DDL.
func QuotePGIdent(s string) string { return pgQuoteIdent(s) }

func mysqlQuoteIdent(s string) string {
	s = strings.ReplaceAll(s, "`", "``")
	return "`" + s + "`"
}

// QuoteMySQLIdent returns a MySQL backtick-quoted identifier for DDL.
func QuoteMySQLIdent(s string) string { return mysqlQuoteIdent(s) }

func isAlreadyExistsErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "database exists")
}

func (S *SQLDriver) adoptOpenedDriver(db *SQLDriver) {
	if db == nil {
		return
	}
	S.ORM = db.ORM
	S.DriverCredential = db.DriverCredential
}

// EnsureMetaMediaTables creates meta and media tables when missing (idempotent).
func (S *SQLDriver) EnsureMetaMediaTables(ctx context.Context) error {
	switch S.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		return S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.NewRaw(`
			CREATE TABLE IF NOT EXISTS public.meta(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				doc_id VARCHAR(36) NOT NULL,
				created_at DATE NOT NULL DEFAULT CURRENT_DATE,
				updated_at DATE NOT NULL DEFAULT CURRENT_DATE,
				created_by VARCHAR(36) NOT NULL,
				updated_by VARCHAR(36),
				status VARCHAR(36)
			);`).Exec(ctx); err != nil {
				return err
			}
			if _, err := tx.NewRaw(`
			CREATE TABLE IF NOT EXISTS public.media(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				model VARCHAR(125),
				media_type VARCHAR(65),
				file_extension VARCHAR(65),
				file_name TEXT,
				size INTEGER,
				s3_key TEXT,
				url TEXT,
				created_at DATE NOT NULL DEFAULT CURRENT_DATE
			);`).Exec(ctx); err != nil {
				return err
			}
			return nil
		})
	case _const.MySQLDriver, _const.MariaDBDriver:
		return S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.NewRaw(`
			CREATE TABLE IF NOT EXISTS meta(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				doc_id VARCHAR(36) NOT NULL,
				created_at DATE NOT NULL DEFAULT (CURRENT_DATE),
				updated_at DATE NOT NULL DEFAULT (CURRENT_DATE),
				created_by VARCHAR(36) NOT NULL,
				updated_by VARCHAR(36),
				status VARCHAR(35)
			);`).Exec(ctx); err != nil {
				return err
			}
			if _, err := tx.NewRaw(`
			CREATE TABLE IF NOT EXISTS media(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				model VARCHAR(125),
				media_type VARCHAR(65),
				file_extension VARCHAR(65),
				file_name TEXT,
				size INTEGER,
				s3_key TEXT,
				url TEXT,
				created_at DATE NOT NULL DEFAULT (CURRENT_DATE)
			);`).Exec(ctx); err != nil {
				return err
			}
			return nil
		})
	case _const.SQLiteDriver, "libsql":
		return S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.NewRaw(`
			CREATE TABLE IF NOT EXISTS meta(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				doc_id VARCHAR(36) NOT NULL,
				created_at DATE NOT NULL DEFAULT CURRENT_DATE,
				updated_at DATE NOT NULL DEFAULT CURRENT_DATE,
				created_by VARCHAR(36) NOT NULL,
				updated_by VARCHAR(36),
				status VARCHAR(36)
			);`).Exec(ctx); err != nil {
				return err
			}
			if _, err := tx.NewRaw(`
			CREATE TABLE IF NOT EXISTS media(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				model VARCHAR(125),
				media_type VARCHAR(65),
				file_extension VARCHAR(65),
				file_name TEXT,
				size INTEGER,
				s3_key TEXT,
				url TEXT,
				created_at DATE NOT NULL DEFAULT CURRENT_DATE
			);`).Exec(ctx); err != nil {
				return err
			}
			return nil
		})
	default:
		return fmt.Errorf("EnsureMetaMediaTables: unsupported engine %s", S.DriverCredential.Engine)
	}
}

func (S *SQLDriver) metaTableExists(ctx context.Context) (bool, error) {
	switch S.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		var n int
		err := S.ORM.NewRaw(`SELECT COUNT(*)::int FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'meta'`).Scan(ctx, &n)
		return n > 0, err
	case _const.MySQLDriver, _const.MariaDBDriver:
		var n int
		err := S.ORM.NewRaw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'meta'`).Scan(ctx, &n)
		return n > 0, err
	case _const.SQLiteDriver, "libsql":
		var n int
		err := S.ORM.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='meta'`).Scan(ctx, &n)
		return n > 0, err
	default:
		return false, fmt.Errorf("metaTableExists: unsupported engine %s", S.DriverCredential.Engine)
	}
}

func (S *SQLDriver) currentPostgresDatabase(ctx context.Context) (string, error) {
	var cur string
	err := S.ORM.NewRaw(`SELECT current_database()`).Scan(ctx, &cur)
	return strings.TrimSpace(cur), err
}

func (S *SQLDriver) currentMySQLDatabase(ctx context.Context) (string, error) {
	var cur *string
	err := S.ORM.NewRaw(`SELECT DATABASE()`).Scan(ctx, &cur)
	if err != nil || cur == nil {
		return "", err
	}
	return strings.TrimSpace(*cur), err
}

// InitProjectBase creates the per-project database (PostgreSQL / MySQL) when needed, reconnects,
// and ensures meta + media tables. For SQLite / libsql it only ensures meta + media in the current file.
func (S *SQLDriver) InitProjectBase(ctx context.Context, param *models.CommonSystemParams, indexes []string) error {
	if param == nil {
		return errors.New("InitProjectBase: param required")
	}
	pid := strings.TrimSpace(param.ProjectID)
	if pid == "" {
		return errors.New("InitProjectBase: project id required")
	}
	if S.DriverCredential == nil {
		return errors.New("InitProjectBase: nil driver credentials")
	}

	switch S.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		return S.initPostgresProject(ctx, pid)
	case _const.MySQLDriver, _const.MariaDBDriver:
		return S.initMySQLProject(ctx, pid)
	case _const.SQLiteDriver, "libsql":
		return S.EnsureMetaMediaTables(ctx)
	default:
		return fmt.Errorf("InitProjectBase: unsupported engine %s", S.DriverCredential.Engine)
	}
}

func physicalProjectDBName(cred *models.DriverCredentials, logicalPID string) string {
	if cred == nil {
		return strings.TrimSpace(logicalPID)
	}
	d := strings.TrimSpace(cred.Database)
	if d == "" {
		return strings.TrimSpace(logicalPID)
	}
	ld := strings.ToLower(d)
	if ld == "postgres" || ld == "template0" || ld == "template1" || ld == "mysql" || ld == "sys" {
		return strings.TrimSpace(logicalPID)
	}
	return d
}

func (S *SQLDriver) initPostgresProject(ctx context.Context, logicalProjectID string) error {
	dbName := physicalProjectDBName(S.DriverCredential, logicalProjectID)
	ok, err := S.metaTableExists(ctx)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	cur, err := S.currentPostgresDatabase(ctx)
	if err != nil {
		return err
	}
	if cur != dbName {
		if _, err := S.ORM.NewRaw(fmt.Sprintf("CREATE DATABASE %s", pgQuoteIdent(dbName))).Exec(ctx); err != nil && !isAlreadyExistsErr(err) {
			return err
		}
		cred := *S.DriverCredential
		cred.Database = dbName
		db, err := GetSQLDriver(S.Conf, &cred)
		if err != nil {
			return err
		}
		S.adoptOpenedDriver(db)
	}
	return S.EnsureMetaMediaTables(ctx)
}

func (S *SQLDriver) initMySQLProject(ctx context.Context, logicalProjectID string) error {
	dbName := physicalProjectDBName(S.DriverCredential, logicalProjectID)
	ok, err := S.metaTableExists(ctx)
	if err != nil {
		return err
	}
	if ok {
		return nil
	}
	cur, err := S.currentMySQLDatabase(ctx)
	if err != nil {
		return err
	}
	if cur != dbName {
		if _, err := S.ORM.NewRaw(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", mysqlQuoteIdent(dbName))).Exec(ctx); err != nil {
			return err
		}
		cred := *S.DriverCredential
		cred.Database = dbName
		db, err := GetSQLDriver(S.Conf, &cred)
		if err != nil {
			return err
		}
		S.adoptOpenedDriver(db)
	}
	return S.EnsureMetaMediaTables(ctx)
}

// DeleteProjectBase drops meta and media in the current database (SQLite / libsql layout).
func (S *SQLDriver) DeleteProjectBase(ctx context.Context, param *models.CommonSystemParams) error {
	if param == nil || strings.TrimSpace(param.ProjectID) == "" {
		return errors.New("DeleteProjectBase: project id required")
	}
	switch S.DriverCredential.Engine {
	case _const.SQLiteDriver, "libsql":
		_, err := S.ORM.NewRaw(`DROP TABLE IF EXISTS meta; DROP TABLE IF EXISTS media;`).Exec(ctx)
		return err
	default:
		return nil
	}
}

func (S *SQLDriver) dropPostgresDatabase(ctx context.Context, dbName string) error {
	cred := *S.DriverCredential
	cred.Database = "postgres"
	admin, err := GetSQLDriver(S.Conf, &cred)
	if err != nil {
		return err
	}
	defer admin.ORM.Close()
	_, err = admin.ORM.NewRaw(fmt.Sprintf("DROP DATABASE IF EXISTS %s", pgQuoteIdent(dbName))).Exec(ctx)
	return err
}

func (S *SQLDriver) dropMySQLDatabase(ctx context.Context, dbName string) error {
	cred := *S.DriverCredential
	cred.Database = "mysql"
	admin, err := GetSQLDriver(S.Conf, &cred)
	if err != nil {
		return err
	}
	defer admin.ORM.Close()
	_, err = admin.ORM.NewRaw(fmt.Sprintf("DROP DATABASE IF EXISTS %s", mysqlQuoteIdent(dbName))).Exec(ctx)
	return err
}

// DeleteProject removes project storage: dedicated database for PostgreSQL / MySQL, or core tables for SQLite / libsql.
func (S *SQLDriver) DeleteProject(ctx context.Context, projectId string) error {
	pid := strings.TrimSpace(projectId)
	if pid == "" {
		return nil
	}
	switch S.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		_ = S.ORM.Close()
		return S.dropPostgresDatabase(ctx, physicalProjectDBName(S.DriverCredential, pid))
	case _const.MySQLDriver, _const.MariaDBDriver:
		_ = S.ORM.Close()
		return S.dropMySQLDatabase(ctx, physicalProjectDBName(S.DriverCredential, pid))
	case _const.SQLiteDriver, "libsql":
		return S.DeleteProjectBase(ctx, &models.CommonSystemParams{ProjectID: pid})
	default:
		return fmt.Errorf("DeleteProject: unsupported engine %s", S.DriverCredential.Engine)
	}
}

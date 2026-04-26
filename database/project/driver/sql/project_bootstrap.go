package sql

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
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

func (S *SQLDriver) postgresDedicatedSchema() bool {
	return S != nil && S.DriverCredential != nil &&
		S.DriverCredential.Engine == _const.PostgreSQLDriver &&
		strings.TrimSpace(S.DriverCredential.Schema) != ""
}

// execMetaMediaSecondaryDDL creates secondary indexes on meta/media and ensures
// document_revisions exists with an index for list-by-original-doc queries.
// metaQualifier / mediaQualifier / docRevTable are full table names as emitted in DDL
// (e.g. public.meta, `meta`, or meta in a dedicated PG schema).
func execMetaMediaSecondaryDDL(ctx context.Context, tx bun.Tx, metaQualifier, mediaQualifier, docRevTable string) error {
	indexStmts := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_meta_doc_id ON %s(doc_id)`, metaQualifier),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_meta_doc_created ON %s(doc_id, created_at DESC)`, metaQualifier),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_media_model_created ON %s(model, created_at DESC)`, mediaQualifier),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_media_created_at ON %s(created_at DESC)`, mediaQualifier),
	}
	for _, q := range indexStmts {
		if _, err := tx.NewRaw(q).Exec(ctx); err != nil {
			return err
		}
	}
	docRevDDL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s(
			id VARCHAR(36) NOT NULL PRIMARY KEY,
			original_doc_id VARCHAR(36) NOT NULL,
			revision_at VARCHAR(128) NOT NULL,
			status VARCHAR(64)
		);`, docRevTable)
	if _, err := tx.NewRaw(docRevDDL).Exec(ctx); err != nil {
		return err
	}
	revIdx := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_docrev_original_doc_revised ON %s(original_doc_id, revision_at DESC)`, docRevTable)
	if _, err := tx.NewRaw(revIdx).Exec(ctx); err != nil {
		return err
	}
	return nil
}

// EnsureMetaMediaTables creates meta and media tables when missing (idempotent).
func (S *SQLDriver) EnsureMetaMediaTables(ctx context.Context) error {
	switch S.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		return S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			metaDDL := `
			CREATE TABLE IF NOT EXISTS public.meta(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				doc_id VARCHAR(36) NOT NULL,
				created_at DATE NOT NULL DEFAULT CURRENT_DATE,
				updated_at DATE NOT NULL DEFAULT CURRENT_DATE,
				created_by VARCHAR(36) NOT NULL,
				updated_by VARCHAR(36),
				status VARCHAR(36)
			);`
			mediaDDL := `
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
			);`
			if S.postgresDedicatedSchema() {
				metaDDL = `
			CREATE TABLE IF NOT EXISTS meta(
				id VARCHAR(36) NOT NULL PRIMARY KEY,
				doc_id VARCHAR(36) NOT NULL,
				created_at DATE NOT NULL DEFAULT CURRENT_DATE,
				updated_at DATE NOT NULL DEFAULT CURRENT_DATE,
				created_by VARCHAR(36) NOT NULL,
				updated_by VARCHAR(36),
				status VARCHAR(36)
			);`
				mediaDDL = `
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
			);`
			}
			if _, err := tx.NewRaw(metaDDL).Exec(ctx); err != nil {
				return err
			}
			if _, err := tx.NewRaw(mediaDDL).Exec(ctx); err != nil {
				return err
			}
			if S.postgresDedicatedSchema() {
				if err := execMetaMediaSecondaryDDL(ctx, tx, "meta", "media", "document_revisions"); err != nil {
					return err
				}
			} else {
				if err := execMetaMediaSecondaryDDL(ctx, tx, "public.meta", "public.media", "public.document_revisions"); err != nil {
					return err
				}
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
			if err := execMetaMediaSecondaryDDL(ctx, tx, "`meta`", "`media`", "`document_revisions`"); err != nil {
				return err
			}
			return nil
		})
	case _const.SQLiteDriver:
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
			if err := execMetaMediaSecondaryDDL(ctx, tx, "`meta`", "`media`", "`document_revisions`"); err != nil {
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
		var err error
		if sch := strings.TrimSpace(S.DriverCredential.Schema); sch != "" {
			err = S.ORM.NewRaw(`SELECT COUNT(*)::int FROM information_schema.tables WHERE table_schema = ? AND table_name = 'meta'`, sch).Scan(ctx, &n)
		} else {
			err = S.ORM.NewRaw(`SELECT COUNT(*)::int FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'meta'`).Scan(ctx, &n)
		}
		return n > 0, err
	case _const.MySQLDriver, _const.MariaDBDriver:
		var n int
		err := S.ORM.NewRaw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'meta'`).Scan(ctx, &n)
		return n > 0, err
	case _const.SQLiteDriver:
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
// and ensures meta + media tables. For SQLite it only ensures meta + media in the current file.
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
	case _const.SQLiteDriver:
		if err := S.EnsureMetaMediaTables(ctx); err != nil {
			return err
		}
		return S.finishInitProjectBase(ctx)
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
	schemaName := strings.TrimSpace(S.DriverCredential.Schema)
	if schemaName == "" && S.Conf != nil && strings.EqualFold(strings.TrimSpace(S.Conf.GeneralPostgresIsolation), "schema") {
		if strings.Contains(strings.ToLower(strings.TrimSpace(S.DriverCredential.Host)), ".neon.tech") {
			schemaName = ""
		} else {
			schemaName = utility.PostgresProjectSchemaName(logicalProjectID)
		}
	}
	if schemaName != "" {
		return S.initPostgresProjectSchema(ctx, logicalProjectID, schemaName)
	}

	dbName := physicalProjectDBName(S.DriverCredential, logicalProjectID)
	ok, err := S.metaTableExists(ctx)
	if err != nil {
		return err
	}
	if ok {
		return S.finishInitProjectBase(ctx)
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
	if err := S.EnsureMetaMediaTables(ctx); err != nil {
		return err
	}
	return S.finishInitProjectBase(ctx)
}

// initPostgresProjectSchema creates a dedicated schema on the shared Database and reconnects with search_path (DSN options).
func (S *SQLDriver) initPostgresProjectSchema(ctx context.Context, logicalProjectID, schemaName string) error {
	schemaName = strings.TrimSpace(schemaName)
	if schemaName == "" {
		return errors.New("init postgres schema: empty schema name")
	}
	if S.DriverCredential != nil {
		S.DriverCredential.Schema = schemaName
	}
	ok, err := S.metaTableExists(ctx)
	if err != nil {
		return err
	}
	if ok {
		return S.finishInitProjectBase(ctx)
	}

	credNoSchema := *S.DriverCredential
	credNoSchema.Schema = ""
	bootstrap, err := GetSQLDriver(S.Conf, &credNoSchema)
	if err != nil {
		return fmt.Errorf("init postgres schema: bootstrap open: %w", err)
	}
	defer bootstrap.ORM.Close()

	if _, err := bootstrap.ORM.NewRaw(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", pgQuoteIdent(schemaName))).Exec(ctx); err != nil && !isAlreadyExistsErr(err) {
		return fmt.Errorf("init postgres schema: create schema: %w", err)
	}

	credWith := *S.DriverCredential
	credWith.Schema = schemaName
	db, err := GetSQLDriver(S.Conf, &credWith)
	if err != nil {
		return fmt.Errorf("init postgres schema: reopen with schema: %w", err)
	}
	_ = S.ORM.Close()
	S.adoptOpenedDriver(db)
	if err := S.EnsureMetaMediaTables(ctx); err != nil {
		return err
	}
	return S.finishInitProjectBase(ctx)
}

func (S *SQLDriver) initMySQLProject(ctx context.Context, logicalProjectID string) error {
	if S.Conf != nil {
		isol := strings.TrimSpace(S.Conf.GeneralMySQLIsolation)
		if isol != "" && !strings.EqualFold(isol, "database") {
			return fmt.Errorf("unsupported GENERAL_MYSQL_ISOLATION %q (only \"database\" is supported for MySQL/MariaDB)", isol)
		}
	}
	dbName := physicalProjectDBName(S.DriverCredential, logicalProjectID)
	ok, err := S.metaTableExists(ctx)
	if err != nil {
		return err
	}
	if ok {
		return S.finishInitProjectBase(ctx)
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
	if err := S.EnsureMetaMediaTables(ctx); err != nil {
		return err
	}
	return S.finishInitProjectBase(ctx)
}

// DeleteProjectBase drops meta and media in the current database (SQLite file-backed layout).
func (S *SQLDriver) DeleteProjectBase(ctx context.Context, param *models.CommonSystemParams) error {
	if param == nil || strings.TrimSpace(param.ProjectID) == "" {
		return errors.New("DeleteProjectBase: project id required")
	}
	switch S.DriverCredential.Engine {
	case _const.SQLiteDriver:
		_, err := S.ORM.NewRaw(`DROP TABLE IF EXISTS meta; DROP TABLE IF EXISTS media;`).Exec(ctx)
		return err
	default:
		return nil
	}
}

func (S *SQLDriver) dropPostgresDatabase(ctx context.Context, dbName string) error {
	cred := *S.DriverCredential
	cred.Database = "postgres"
	cred.Schema = ""
	admin, err := GetSQLDriver(S.Conf, &cred)
	if err != nil {
		return err
	}
	defer admin.ORM.Close()
	_, err = admin.ORM.NewRaw(fmt.Sprintf("DROP DATABASE IF EXISTS %s", pgQuoteIdent(dbName))).Exec(ctx)
	return err
}

func (S *SQLDriver) dropPostgresSchema(ctx context.Context, schemaName string) error {
	cred := *S.DriverCredential
	cred.Schema = ""
	admin, err := GetSQLDriver(S.Conf, &cred)
	if err != nil {
		return err
	}
	defer admin.ORM.Close()
	_, err = admin.ORM.NewRaw(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", pgQuoteIdent(schemaName))).Exec(ctx)
	return err
}

func (S *SQLDriver) dropMySQLDatabase(ctx context.Context, dbName string) error {
	cred := *S.DriverCredential
	cred.Database = "mysql"
	cred.Schema = ""
	admin, err := GetSQLDriver(S.Conf, &cred)
	if err != nil {
		return err
	}
	defer admin.ORM.Close()
	_, err = admin.ORM.NewRaw(fmt.Sprintf("DROP DATABASE IF EXISTS %s", mysqlQuoteIdent(dbName))).Exec(ctx)
	return err
}

// DeleteProject removes project storage: dedicated database for PostgreSQL / MySQL, or core tables for SQLite.
func (S *SQLDriver) DeleteProject(ctx context.Context, projectId string) error {
	pid := strings.TrimSpace(projectId)
	if pid == "" {
		return nil
	}
	switch S.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		_ = S.ORM.Close()
		if sch := strings.TrimSpace(S.DriverCredential.Schema); sch != "" {
			return S.dropPostgresSchema(ctx, sch)
		}
		return S.dropPostgresDatabase(ctx, physicalProjectDBName(S.DriverCredential, pid))
	case _const.MySQLDriver, _const.MariaDBDriver:
		_ = S.ORM.Close()
		return S.dropMySQLDatabase(ctx, physicalProjectDBName(S.DriverCredential, pid))
	case _const.SQLiteDriver:
		if err := S.DeleteProjectBase(ctx, &models.CommonSystemParams{ProjectID: pid}); err != nil {
			return err
		}
		// Per-project SQLite files: drop tables then remove the file (shared template DB is untouched).
		if S.Conf != nil && S.Conf.GeneralSQLiteFilePerProject {
			if f := strings.TrimSpace(S.DriverCredential.File); f != "" {
				_ = S.ORM.Close()
				dbPath := filepath.Join(S.Conf.DefaultDatabaseDir, f)
				var expErr error
				dbPath, expErr = utility.ExpandPath(dbPath)
				if expErr == nil {
					_ = os.Remove(dbPath)
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("DeleteProject: unsupported engine %s", S.DriverCredential.Engine)
	}
}

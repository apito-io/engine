package postgres

import (
	"github.com/apito-io/engine/database/project/sqlcommon"
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





func (d *Driver) postgresDedicatedSchema() bool {
	return d != nil && d.DriverCredential != nil &&
		d.DriverCredential.Engine == _const.PostgreSQLDriver &&
		strings.TrimSpace(d.DriverCredential.Schema) != ""
}

// execMetaFilesSecondaryDDL creates secondary indexes on meta/files and ensures
// document_revisions exists with an index for list-by-original-doc queries.
func execMetaFilesSecondaryDDL(ctx context.Context, tx bun.Tx, metaQualifier, filesQualifier, docRevTable string) error {
	indexStmts := []string{
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_meta_doc_id ON %s(doc_id)`, metaQualifier),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_meta_doc_created ON %s(doc_id, created_at DESC)`, metaQualifier),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_files_type_created ON %s(file_type, created_at DESC)`, filesQualifier),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_files_created_at ON %s(created_at DESC)`, filesQualifier),
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

// EnsureMetaFilesTables creates meta and files tables when missing (idempotent).
func (d *Driver) EnsureMetaFilesTables(ctx context.Context) error {
	switch d.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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
			filesDDL := filesTableDDLPostgresPublic
			if d.postgresDedicatedSchema() {
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
				filesDDL = filesTableDDL
			}
			if _, err := tx.NewRaw(metaDDL).Exec(ctx); err != nil {
				return err
			}
			if _, err := tx.NewRaw(filesDDL).Exec(ctx); err != nil {
				return err
			}
			if d.postgresDedicatedSchema() {
				if err := execMetaFilesSecondaryDDL(ctx, tx, "meta", "files", "document_revisions"); err != nil {
					return err
				}
			} else {
				if err := execMetaFilesSecondaryDDL(ctx, tx, "public.meta", "public.files", "public.document_revisions"); err != nil {
					return err
				}
			}
			return nil
		})
	case _const.MySQLDriver, _const.MariaDBDriver:
		return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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
			if _, err := tx.NewRaw(filesTableDDL).Exec(ctx); err != nil {
				return err
			}
			if err := execMetaFilesSecondaryDDL(ctx, tx, "`meta`", "`files`", "`document_revisions`"); err != nil {
				return err
			}
			return nil
		})
	case _const.SQLiteDriver:
		if d.IsRemoteSQLiteLikeTurso() {
			return d.ensureMetaFilesTablesSQLiteRemote(ctx)
		}
		return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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
			if _, err := tx.NewRaw(filesTableDDL).Exec(ctx); err != nil {
				return err
			}
			if err := execMetaFilesSecondaryDDL(ctx, tx, "`meta`", "`files`", "`document_revisions`"); err != nil {
				return err
			}
			return nil
		})
	default:
		return fmt.Errorf("EnsureMetaFilesTables: unsupported engine %s", d.DriverCredential.Engine)
	}
}

// EnsureMetaMediaTables is deprecated; use EnsureMetaFilesTables.
func (d *Driver) EnsureMetaMediaTables(ctx context.Context) error {
	return d.EnsureMetaFilesTables(ctx)
}

// ensureMetaFilesTablesSQLiteRemote runs meta/files DDL on remote Turso without RunInTx
// (hrana pipeline + BEGIN has been observed to surface HTTP 502 during bootstrap).
func (d *Driver) ensureMetaFilesTablesSQLiteRemote(ctx context.Context) error {
	if _, err := d.ORM.NewRaw(`
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
	if _, err := d.ORM.NewRaw(filesTableDDL).Exec(ctx); err != nil {
		return err
	}
	return execMetaFilesSecondaryDDLRemote(ctx, d.ORM)
}

func execMetaFilesSecondaryDDLRemote(ctx context.Context, db *bun.DB) error {
	indexStmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_meta_doc_id ON ` + "`meta`" + `(doc_id)`,
		`CREATE INDEX IF NOT EXISTS idx_meta_doc_created ON ` + "`meta`" + `(doc_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_files_type_created ON ` + "`files`" + `(file_type, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_files_created_at ON ` + "`files`" + `(created_at DESC)`,
	}
	for _, q := range indexStmts {
		if _, err := db.NewRaw(q).Exec(ctx); err != nil {
			return err
		}
	}
	docRevDDL := `
		CREATE TABLE IF NOT EXISTS ` + "`document_revisions`" + `(
			id VARCHAR(36) NOT NULL PRIMARY KEY,
			original_doc_id VARCHAR(36) NOT NULL,
			revision_at VARCHAR(128) NOT NULL,
			status VARCHAR(64)
		);`
	if _, err := db.NewRaw(docRevDDL).Exec(ctx); err != nil {
		return err
	}
	revIdx := `CREATE INDEX IF NOT EXISTS idx_docrev_original_doc_revised ON ` + "`document_revisions`" + `(original_doc_id, revision_at DESC)`
	if _, err := db.NewRaw(revIdx).Exec(ctx); err != nil {
		return err
	}
	return nil
}

func (d *Driver) metaTableExists(ctx context.Context) (bool, error) {
	switch d.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		var n int
		var err error
		if sch := strings.TrimSpace(d.DriverCredential.Schema); sch != "" {
			err = d.ORM.NewRaw(`SELECT COUNT(*)::int FROM information_schema.tables WHERE table_schema = ? AND table_name = 'meta'`, sch).Scan(ctx, &n)
		} else {
			err = d.ORM.NewRaw(`SELECT COUNT(*)::int FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'meta'`).Scan(ctx, &n)
		}
		return n > 0, err
	case _const.MySQLDriver, _const.MariaDBDriver:
		var n int
		err := d.ORM.NewRaw(`SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'meta'`).Scan(ctx, &n)
		return n > 0, err
	case _const.SQLiteDriver:
		var n int
		err := d.ORM.NewRaw(`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='meta'`).Scan(ctx, &n)
		return n > 0, err
	default:
		return false, fmt.Errorf("metaTableExists: unsupported engine %s", d.DriverCredential.Engine)
	}
}

func (d *Driver) currentPostgresDatabase(ctx context.Context) (string, error) {
	var cur string
	err := d.ORM.NewRaw(`SELECT current_database()`).Scan(ctx, &cur)
	return strings.TrimSpace(cur), err
}

func (d *Driver) currentMySQLDatabase(ctx context.Context) (string, error) {
	var cur *string
	err := d.ORM.NewRaw(`SELECT DATABASE()`).Scan(ctx, &cur)
	if err != nil || cur == nil {
		return "", err
	}
	return strings.TrimSpace(*cur), err
}

// InitProjectBase creates the per-project database (PostgreSQL / MySQL) when needed, reconnects,
// and ensures meta + files tables. For SQLite it only ensures meta + files in the current file.
func (d *Driver) InitProjectBase(ctx context.Context, param *models.CommonSystemParams, indexes []string) error {
	if param == nil {
		return errors.New("InitProjectBase: param required")
	}
	pid := strings.TrimSpace(param.ProjectID)
	if pid == "" {
		return errors.New("InitProjectBase: project id required")
	}
	if d.DriverCredential == nil {
		return errors.New("InitProjectBase: nil driver credentials")
	}

	switch d.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		return d.initPostgresProject(ctx, pid)
	case _const.MySQLDriver, _const.MariaDBDriver:
		return d.initMySQLProject(ctx, pid)
	case _const.SQLiteDriver:
		if err := d.EnsureMetaFilesTables(ctx); err != nil {
			return err
		}
		return d.finishInitProjectBase(ctx)
	default:
		return fmt.Errorf("InitProjectBase: unsupported engine %s", d.DriverCredential.Engine)
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

func (d *Driver) initPostgresProject(ctx context.Context, logicalProjectID string) error {
	schemaName := strings.TrimSpace(d.DriverCredential.Schema)
	if schemaName == "" && d.Conf != nil && strings.EqualFold(strings.TrimSpace(d.Conf.GeneralPostgresIsolation), "schema") {
		if strings.Contains(strings.ToLower(strings.TrimSpace(d.DriverCredential.Host)), ".neon.tech") {
			schemaName = ""
		} else {
			schemaName = utility.PostgresProjectSchemaName(logicalProjectID)
		}
	}
	if schemaName != "" {
		return d.initPostgresProjectSchema(ctx, logicalProjectID, schemaName)
	}

	dbName := physicalProjectDBName(d.DriverCredential, logicalProjectID)
	ok, err := d.metaTableExists(ctx)
	if err != nil {
		return err
	}
	if ok {
		return d.finishInitProjectBase(ctx)
	}
	cur, err := d.currentPostgresDatabase(ctx)
	if err != nil {
		return err
	}
	if cur != dbName {
		if _, err := d.ORM.NewRaw(fmt.Sprintf("CREATE DATABASE %s", sqlcommon.QuotePGIdent(dbName))).Exec(ctx); err != nil && !sqlcommon.IsAlreadyExistsErr(err) {
			return err
		}
		cred := *d.DriverCredential
		cred.Database = dbName
		db, err := openBootstrapBun(d.Conf, &cred)
		if err != nil {
			return err
		}
		d.adoptOpenedDriver(db)
	}
	if err := d.EnsureMetaFilesTables(ctx); err != nil {
		return err
	}
	if err := d.EnsureUsersTable(ctx); err != nil {
		return err
	}
	return d.finishInitProjectBase(ctx)
}

// initPostgresProjectSchema creates a dedicated schema on the shared Database and reconnects with search_path (DSN options).
func (d *Driver) initPostgresProjectSchema(ctx context.Context, logicalProjectID, schemaName string) error {
	schemaName = strings.TrimSpace(schemaName)
	if schemaName == "" {
		return errors.New("init postgres schema: empty schema name")
	}
	if d.DriverCredential != nil {
		d.DriverCredential.Schema = schemaName
	}
	ok, err := d.metaTableExists(ctx)
	if err != nil {
		return err
	}
	if ok {
		return d.finishInitProjectBase(ctx)
	}

	credNoSchema := *d.DriverCredential
	credNoSchema.Schema = ""
	bootstrap, err := openBootstrapBun(d.Conf, &credNoSchema)
	if err != nil {
		return fmt.Errorf("init postgres schema: bootstrap open: %w", err)
	}
	defer bootstrap.ORM.Close()

	if _, err := bootstrap.ORM.NewRaw(fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s", sqlcommon.QuotePGIdent(schemaName))).Exec(ctx); err != nil && !sqlcommon.IsAlreadyExistsErr(err) {
		return fmt.Errorf("init postgres schema: create schema: %w", err)
	}

	credWith := *d.DriverCredential
	credWith.Schema = schemaName
	db, err := openBootstrapBun(d.Conf, &credWith)
	if err != nil {
		return fmt.Errorf("init postgres schema: reopen with schema: %w", err)
	}
	_ = d.ORM.Close()
	d.adoptOpenedDriver(db)
	if err := d.EnsureMetaFilesTables(ctx); err != nil {
		return err
	}
	if err := d.EnsureUsersTable(ctx); err != nil {
		return err
	}
	return d.finishInitProjectBase(ctx)
}

func (d *Driver) initMySQLProject(ctx context.Context, logicalProjectID string) error {
	if d.Conf != nil {
		isol := strings.TrimSpace(d.Conf.GeneralMySQLIsolation)
		if isol != "" && !strings.EqualFold(isol, "database") {
			return fmt.Errorf("unsupported GENERAL_MYSQL_ISOLATION %q (only \"database\" is supported for MySQL/MariaDB)", isol)
		}
	}
	dbName := physicalProjectDBName(d.DriverCredential, logicalProjectID)
	ok, err := d.metaTableExists(ctx)
	if err != nil {
		return err
	}
	if ok {
		return d.finishInitProjectBase(ctx)
	}
	cur, err := d.currentMySQLDatabase(ctx)
	if err != nil {
		return err
	}
	if cur != dbName {
		if _, err := d.ORM.NewRaw(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", sqlcommon.QuoteMySQLIdent(dbName))).Exec(ctx); err != nil {
			return err
		}
		cred := *d.DriverCredential
		cred.Database = dbName
		db, err := openBootstrapBun(d.Conf, &cred)
		if err != nil {
			return err
		}
		d.adoptOpenedDriver(db)
	}
	if err := d.EnsureMetaFilesTables(ctx); err != nil {
		return err
	}
	if err := d.EnsureUsersTable(ctx); err != nil {
		return err
	}
	return d.finishInitProjectBase(ctx)
}

// DeleteProjectBase drops meta and files in the current database (SQLite file-backed layout).
func (d *Driver) DeleteProjectBase(ctx context.Context, param *models.CommonSystemParams) error {
	if param == nil || strings.TrimSpace(param.ProjectID) == "" {
		return errors.New("DeleteProjectBase: project id required")
	}
	switch d.DriverCredential.Engine {
	case _const.SQLiteDriver:
		_, err := d.ORM.NewRaw(`DROP TABLE IF EXISTS meta; DROP TABLE IF EXISTS files; DROP TABLE IF EXISTS media;`).Exec(ctx)
		return err
	default:
		return nil
	}
}

func (d *Driver) dropPostgresDatabase(ctx context.Context, dbName string) error {
	cred := *d.DriverCredential
	cred.Database = "postgres"
	cred.Schema = ""
	admin, err := openBootstrapBun(d.Conf, &cred)
	if err != nil {
		return err
	}
	defer admin.ORM.Close()
	_, err = admin.ORM.NewRaw(fmt.Sprintf("DROP DATABASE IF EXISTS %s", sqlcommon.QuotePGIdent(dbName))).Exec(ctx)
	return err
}

func (d *Driver) dropPostgresSchema(ctx context.Context, schemaName string) error {
	cred := *d.DriverCredential
	cred.Schema = ""
	admin, err := openBootstrapBun(d.Conf, &cred)
	if err != nil {
		return err
	}
	defer admin.ORM.Close()
	_, err = admin.ORM.NewRaw(fmt.Sprintf("DROP SCHEMA IF EXISTS %s CASCADE", sqlcommon.QuotePGIdent(schemaName))).Exec(ctx)
	return err
}

func (d *Driver) dropMySQLDatabase(ctx context.Context, dbName string) error {
	cred := *d.DriverCredential
	cred.Database = "mysql"
	cred.Schema = ""
	admin, err := openBootstrapBun(d.Conf, &cred)
	if err != nil {
		return err
	}
	defer admin.ORM.Close()
	_, err = admin.ORM.NewRaw(fmt.Sprintf("DROP DATABASE IF EXISTS %s", sqlcommon.QuoteMySQLIdent(dbName))).Exec(ctx)
	return err
}

// DeleteProject removes project storage: dedicated database for PostgreSQL / MySQL, or core tables for SQLite.
func (d *Driver) DeleteProject(ctx context.Context, projectId string) error {
	pid := strings.TrimSpace(projectId)
	if pid == "" {
		return nil
	}
	switch d.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		_ = d.ORM.Close()
		if sch := strings.TrimSpace(d.DriverCredential.Schema); sch != "" {
			return d.dropPostgresSchema(ctx, sch)
		}
		return d.dropPostgresDatabase(ctx, physicalProjectDBName(d.DriverCredential, pid))
	case _const.MySQLDriver, _const.MariaDBDriver:
		_ = d.ORM.Close()
		return d.dropMySQLDatabase(ctx, physicalProjectDBName(d.DriverCredential, pid))
	case _const.SQLiteDriver:
		if err := d.DeleteProjectBase(ctx, &models.CommonSystemParams{ProjectID: pid}); err != nil {
			return err
		}
		// Per-project SQLite files: drop tables then remove the file (shared template DB is untouched).
		if d.Conf != nil && d.Conf.GeneralSQLiteFilePerProject {
			if f := strings.TrimSpace(d.DriverCredential.File); f != "" {
				_ = d.ORM.Close()
				dbPath := filepath.Join(d.Conf.DefaultDatabaseDir, f)
				var expErr error
				dbPath, expErr = utility.ExpandPath(dbPath)
				if expErr == nil {
					_ = os.Remove(dbPath)
				}
			}
		}
		return nil
	default:
		return fmt.Errorf("DeleteProject: unsupported engine %s", d.DriverCredential.Engine)
	}
}

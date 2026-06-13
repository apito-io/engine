package mariadb

import (
	"context"

	_const "github.com/apito-io/engine/const"
	"github.com/uptrace/bun"
)

const schemaOptV1Key = "schema_opt_v1"

// EnsureSchemaOptimizationsV1 applies idempotent secondary indexes and document_revisions
// bootstrap for project databases created before meta/media secondary indexes existed.
// Uses _apito_db_meta sentinel so work runs at most once per database.
func (d *Driver) EnsureSchemaOptimizationsV1(ctx context.Context) error {
	if d == nil || d.ORM == nil || d.DriverCredential == nil {
		return nil
	}
	switch d.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		return d.ensureSchemaOptV1Postgres(ctx)
	case _const.MySQLDriver, _const.MariaDBDriver:
		return d.ensureSchemaOptV1MySQL(ctx)
	case _const.SQLiteDriver:
		return d.ensureSchemaOptV1SQLite(ctx)
	default:
		return nil
	}
}

func (d *Driver) ensureSchemaOptV1Postgres(ctx context.Context) error {
	return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		metaDDL := `CREATE TABLE IF NOT EXISTS public._apito_db_meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);`
		if d.postgresDedicatedSchema() {
			metaDDL = `CREATE TABLE IF NOT EXISTS _apito_db_meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);`
		}
		if _, err := tx.NewRaw(metaDDL).Exec(ctx); err != nil {
			return err
		}
		var n int
		qSel := `SELECT COUNT(*)::int FROM public._apito_db_meta WHERE k = ? AND v = '1'`
		if d.postgresDedicatedSchema() {
			qSel = `SELECT COUNT(*)::int FROM _apito_db_meta WHERE k = ? AND v = '1'`
		}
		if err := tx.NewRaw(qSel, schemaOptV1Key).Scan(ctx, &n); err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		var metaQ, filesQ, docQ string
		if d.postgresDedicatedSchema() {
			metaQ, filesQ, docQ = "meta", "files", "document_revisions"
		} else {
			metaQ, filesQ, docQ = "public.meta", "public.files", "public.document_revisions"
		}
		if err := execMetaFilesSecondaryDDL(ctx, tx, metaQ, filesQ, docQ); err != nil {
			return err
		}
		qIns := `INSERT INTO public._apito_db_meta (k, v) VALUES (?, ?) ON CONFLICT (k) DO UPDATE SET v = EXCLUDED.v`
		if d.postgresDedicatedSchema() {
			qIns = `INSERT INTO _apito_db_meta (k, v) VALUES (?, ?) ON CONFLICT (k) DO UPDATE SET v = EXCLUDED.v`
		}
		if _, err := tx.NewRaw(qIns, schemaOptV1Key, "1").Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}

func (d *Driver) ensureSchemaOptV1MySQL(ctx context.Context) error {
	return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw(`
			CREATE TABLE IF NOT EXISTS _apito_db_meta (
				k VARCHAR(255) NOT NULL PRIMARY KEY,
				v VARCHAR(255) NOT NULL
			);`).Exec(ctx); err != nil {
			return err
		}
		var n int64
		if err := tx.NewRaw(`SELECT COUNT(*) FROM _apito_db_meta WHERE k = ? AND v = '1'`, schemaOptV1Key).Scan(ctx, &n); err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		if err := execMetaFilesSecondaryDDL(ctx, tx, "`meta`", "`files`", "`document_revisions`"); err != nil {
			return err
		}
		if _, err := tx.NewRaw(`INSERT INTO _apito_db_meta (k, v) VALUES (?, ?) ON DUPLICATE KEY UPDATE v = VALUES(v)`,
			schemaOptV1Key, "1").Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}

func (d *Driver) ensureSchemaOptV1SQLite(ctx context.Context) error {
	return d.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewRaw(`
			CREATE TABLE IF NOT EXISTS _apito_db_meta (
				k TEXT PRIMARY KEY,
				v TEXT NOT NULL
			);`).Exec(ctx); err != nil {
			return err
		}
		var n int
		if err := tx.NewRaw(`SELECT COUNT(*) FROM _apito_db_meta WHERE k = ? AND v = '1'`, schemaOptV1Key).Scan(ctx, &n); err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		if err := execMetaFilesSecondaryDDL(ctx, tx, "`meta`", "`files`", "`document_revisions`"); err != nil {
			return err
		}
		if _, err := tx.NewRaw(`INSERT OR REPLACE INTO _apito_db_meta (k, v) VALUES (?, ?)`, schemaOptV1Key, "1").Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}

// ensureSchemaOptV1AfterMeta runs EnsureSchemaOptimizationsV1 when meta already exists (legacy DBs).
func (d *Driver) ensureSchemaOptV1AfterMeta(ctx context.Context) error {
	ok, err := d.metaTableExists(ctx)
	if err != nil || !ok {
		return err
	}
	return d.EnsureSchemaOptimizationsV1(ctx)
}

func (d *Driver) finishInitProjectBase(ctx context.Context) error {
	if err := d.ensureSchemaOptV1AfterMeta(ctx); err != nil {
		return err
	}
	return RunSQLiteLikePostDDL(ctx, d)
}

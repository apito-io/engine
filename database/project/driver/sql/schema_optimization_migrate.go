package sql

import (
	"context"

	_const "github.com/apito-io/engine/const"
	"github.com/uptrace/bun"
)

const schemaOptV1Key = "schema_opt_v1"

// EnsureSchemaOptimizationsV1 applies idempotent secondary indexes and document_revisions
// bootstrap for project databases created before meta/media secondary indexes existed.
// Uses _apito_db_meta sentinel so work runs at most once per database.
func (S *SQLDriver) EnsureSchemaOptimizationsV1(ctx context.Context) error {
	if S == nil || S.ORM == nil || S.DriverCredential == nil {
		return nil
	}
	switch S.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		return S.ensureSchemaOptV1Postgres(ctx)
	case _const.MySQLDriver, _const.MariaDBDriver:
		return S.ensureSchemaOptV1MySQL(ctx)
	case _const.SQLiteDriver:
		return S.ensureSchemaOptV1SQLite(ctx)
	default:
		return nil
	}
}

func (S *SQLDriver) ensureSchemaOptV1Postgres(ctx context.Context) error {
	return S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		metaDDL := `CREATE TABLE IF NOT EXISTS public._apito_db_meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);`
		if S.postgresDedicatedSchema() {
			metaDDL = `CREATE TABLE IF NOT EXISTS _apito_db_meta (k TEXT PRIMARY KEY, v TEXT NOT NULL);`
		}
		if _, err := tx.NewRaw(metaDDL).Exec(ctx); err != nil {
			return err
		}
		var n int
		qSel := `SELECT COUNT(*)::int FROM public._apito_db_meta WHERE k = ? AND v = '1'`
		if S.postgresDedicatedSchema() {
			qSel = `SELECT COUNT(*)::int FROM _apito_db_meta WHERE k = ? AND v = '1'`
		}
		if err := tx.NewRaw(qSel, schemaOptV1Key).Scan(ctx, &n); err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
		var metaQ, mediaQ, docQ string
		if S.postgresDedicatedSchema() {
			metaQ, mediaQ, docQ = "meta", "media", "document_revisions"
		} else {
			metaQ, mediaQ, docQ = "public.meta", "public.media", "public.document_revisions"
		}
		if err := execMetaMediaSecondaryDDL(ctx, tx, metaQ, mediaQ, docQ); err != nil {
			return err
		}
		qIns := `INSERT INTO public._apito_db_meta (k, v) VALUES (?, ?) ON CONFLICT (k) DO UPDATE SET v = EXCLUDED.v`
		if S.postgresDedicatedSchema() {
			qIns = `INSERT INTO _apito_db_meta (k, v) VALUES (?, ?) ON CONFLICT (k) DO UPDATE SET v = EXCLUDED.v`
		}
		if _, err := tx.NewRaw(qIns, schemaOptV1Key, "1").Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}

func (S *SQLDriver) ensureSchemaOptV1MySQL(ctx context.Context) error {
	return S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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
		if err := execMetaMediaSecondaryDDL(ctx, tx, "`meta`", "`media`", "`document_revisions`"); err != nil {
			return err
		}
		if _, err := tx.NewRaw(`INSERT INTO _apito_db_meta (k, v) VALUES (?, ?) ON DUPLICATE KEY UPDATE v = VALUES(v)`,
			schemaOptV1Key, "1").Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}

func (S *SQLDriver) ensureSchemaOptV1SQLite(ctx context.Context) error {
	return S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
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
		if err := execMetaMediaSecondaryDDL(ctx, tx, "`meta`", "`media`", "`document_revisions`"); err != nil {
			return err
		}
		if _, err := tx.NewRaw(`INSERT OR REPLACE INTO _apito_db_meta (k, v) VALUES (?, ?)`, schemaOptV1Key, "1").Exec(ctx); err != nil {
			return err
		}
		return nil
	})
}

// ensureSchemaOptV1AfterMeta runs EnsureSchemaOptimizationsV1 when meta already exists (legacy DBs).
func (S *SQLDriver) ensureSchemaOptV1AfterMeta(ctx context.Context) error {
	ok, err := S.metaTableExists(ctx)
	if err != nil || !ok {
		return err
	}
	return S.EnsureSchemaOptimizationsV1(ctx)
}

func (S *SQLDriver) finishInitProjectBase(ctx context.Context) error {
	if err := S.ensureSchemaOptV1AfterMeta(ctx); err != nil {
		return err
	}
	return RunSQLiteLikePostDDL(ctx, S)
}

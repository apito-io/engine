package mariadb

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/stretchr/testify/require"
	"github.com/tailor-platform/graphql"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"

	sysdriver "github.com/apito-io/engine/database/system/sqlite"
	"github.com/apito-io/engine/database/system/sqlcommon"
)

func explainDetailContainsUsingIndex(t *testing.T, ctx context.Context, db *bun.DB, query string, args ...interface{}) string {
	t.Helper()
	rows, err := db.QueryContext(ctx, "EXPLAIN QUERY PLAN "+query, args...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rows.Close() })
	var b strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &notused, &detail))
		b.WriteString(detail)
		b.WriteByte('\n')
	}
	require.NoError(t, rows.Err())
	s := strings.ToLower(b.String())
	require.True(t,
		strings.Contains(s, "using index") || strings.Contains(s, "using covering index"),
		"plan should reference an index:\n%s", b.String())
	return b.String()
}

func TestExplainQueryPlansUseIndexes_ProjectSQLite(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())

	_, err = db.NewRaw(`
CREATE TABLE meta (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  doc_id VARCHAR(36) NOT NULL,
  created_at TEXT NOT NULL DEFAULT (date('now')),
  updated_at TEXT NOT NULL DEFAULT (date('now')),
  created_by VARCHAR(36) NOT NULL,
  updated_by VARCHAR(36),
  status VARCHAR(36)
);
CREATE TABLE files (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  project_id VARCHAR(36),
  file_type VARCHAR(32) NOT NULL,
  file_name TEXT NOT NULL,
  created_at VARCHAR(128)
);
CREATE TABLE post (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  title TEXT,
  name TEXT
);
CREATE TABLE tag (
  id VARCHAR(36) NOT NULL PRIMARY KEY,
  name TEXT
);
CREATE TABLE post_tag (
  post_id VARCHAR(36) NOT NULL REFERENCES post(id) ON DELETE CASCADE,
  tag_id VARCHAR(36) NOT NULL REFERENCES tag(id) ON DELETE CASCADE,
  PRIMARY KEY (post_id, tag_id)
);
`).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return execMetaFilesSecondaryDDL(ctx, tx, "`meta`", "`files`", "`document_revisions`")
	}))
	_, err = db.NewRaw(`
CREATE INDEX IF NOT EXISTS idx_post_tag_post_id ON post_tag (post_id);
CREATE INDEX IF NOT EXISTS idx_post_tag_tag_id ON post_tag (tag_id);
`).Exec(ctx)
	require.NoError(t, err)
	_, _ = db.NewRaw("ANALYZE").Exec(ctx)

	cfg := &models.Config{}
	cred := &models.DriverCredentials{Engine: _const.SQLiteDriver}
	_ = testDriverWithConf(db, cfg, cred)

	postModel := &models.ModelType{Name: "post", Fields: []*models.FieldInfo{
		{Identifier: "title", FieldType: _const.TextField},
	}}
	tagModel := &models.ModelType{
		Name: "tag",
		Fields: []*models.FieldInfo{
			{Identifier: "name", FieldType: _const.TextField},
		},
		Connections: []*models.ConnectionType{
			{Model: "post", Relation: "has_many", Type: "backward"},
		},
	}

	// List + count (meta join + ORDER BY meta.created_at)
	rp := &graphql.ResolveParams{
		Args: map[string]interface{}{"limit": 10, "start": 0, "local": "en"},
		Info: graphql.ResolveInfo{FieldName: "post"},
	}
	param := &models.CommonSystemParams{
		Model:         postModel,
		ResolveParams: rp,
	}
	qList, argsList, err := RootResolverQueryBuilder(cfg, param, false)
	require.NoError(t, err)
	explainDetailContainsUsingIndex(t, ctx, db, qList, argsList...)

	qCount, argsCount, err := RootResolverQueryBuilder(cfg, param, true)
	require.NoError(t, err)
	explainDetailContainsUsingIndex(t, ctx, db, qCount, argsCount...)

	// Single document + meta
	hookWhere := " AND y.status = ?"
	qSingle := "SELECT x.id AS id, x.title AS title, y.created_at AS sys_created_at FROM `post` AS x LEFT JOIN meta as y on y.doc_id = x.id WHERE x.id = ?" + hookWhere
	explainDetailContainsUsingIndex(t, ctx, db, qSingle, "p1", "live")

	// M2M relation batch
	relParam := &models.CommonSystemParams{
		Model:         tagModel,
		DocumentIDs:   []string{"p1"},
		OnlyReturnCount: false,
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{
				"local":         "en",
				"from_model":    "tag",
				"to_model":      "post",
				"relation_type": "has_many",
			},
		},
	}
	qRel, relArgs, _, err := BuildCombinedRelationQuery(cfg, "", utility.PhysicalSQLTableName("post"), relParam)
	require.NoError(t, err)
	explainDetailContainsUsingIndex(t, ctx, db, qRel, relArgs...)

	// Revisions list pattern
	qRev := `SELECT id FROM document_revisions WHERE original_doc_id = ? OR id = ? ORDER BY revision_at DESC`
	explainDetailContainsUsingIndex(t, ctx, db, qRev, "d1", "d1")

	// Files list pattern (indexes from execMetaFilesSecondaryDDL)
	qFiles := `SELECT * FROM files AS x WHERE x.file_type = 'media' ORDER BY x.created_at DESC LIMIT 10 OFFSET 0`
	explainDetailContainsUsingIndex(t, ctx, db, qFiles)
}

func TestExplainQueryPlansUseIndexes_SystemSQLite(t *testing.T) {
	ctx := context.Background()
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	sys := &sysdriver.Driver{
		Base: sqlcommon.Base{
			Conf:             &models.Config{},
			ORM:              db,
			DriverCredential: &models.DriverCredentials{Engine: _const.SQLiteDriver},
		},
	}
	require.NoError(t, sys.RunMigration(ctx))
	q := `SELECT id FROM audit_logs WHERE project_id = ? ORDER BY created_at DESC LIMIT 20`
	explainDetailContainsUsingIndex(t, ctx, db, q, "00000000-0000-0000-0000-000000000001")
}

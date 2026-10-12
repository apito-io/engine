package starterdemo

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/open_driver/system/bootstrapmeta"
	systemsqlite "github.com/apito-io/open_driver/system/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestEnsureSeedsTodoNotesOnce(t *testing.T) {
	dir := t.TempDir()
	cfg := &models.Config{
		SystemDatabaseEngine: "sqlite",
		DefaultDatabaseDir:   dir,
	}
	systemDB, err := systemsqlite.GetSQLiteSystemDriver(cfg, &models.DriverCredentials{
		Engine: "sqlite",
		File:   "system.sqlite",
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = systemDB.Close() })

	ctx := context.Background()
	require.NoError(t, systemDB.RunMigration(ctx))
	require.NoError(t, systemDB.EnsureSystemBootstrap(ctx))
	require.NoError(t, Ensure(ctx, cfg, systemDB))

	project, err := systemDB.GetProject(ctx, bootstrapmeta.StarterProjectID)
	require.NoError(t, err)
	require.Equal(t, "Todo Notes", project.Name)
	require.Equal(t, "A sample notebook with lists, notes, and tags you can open immediately.", project.Description)

	names := map[string]bool{}
	for _, model := range project.Schema.Models {
		if model == nil {
			continue
		}
		names[model.Name] = true
		require.NotEmpty(t, model.Fields)
	}
	require.True(t, names["notebook"])
	require.True(t, names["note"])
	require.True(t, names["tag"])

	db, err := sql.Open(sqliteshim.ShimName, filepath.Join(dir, bootstrapmeta.OSSStarterSQLiteFile))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var notes int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM note`).Scan(&notes))
	require.Equal(t, 8, notes)
	var linked int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM note WHERE notebook_id IS NOT NULL AND notebook_id != ''`).Scan(&linked))
	require.Equal(t, 8, linked)
	var tags int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM note_tag`).Scan(&tags))
	require.GreaterOrEqual(t, tags, 6)
	var meta int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM meta`).Scan(&meta))
	require.GreaterOrEqual(t, meta, 8)

	require.NoError(t, Ensure(ctx, cfg, systemDB))
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM note`).Scan(&notes))
	require.Equal(t, 8, notes)
}

package sql

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/apito-io/engine/database/system/bootstrapmeta"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
)

func TestEnsureSystemBootstrap_SeedsAdminAndStarterGraph(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := &SystemSQLDriver{
		ORM: db,
		Conf: &models.Config{
			SystemDatabaseEngine: "coredb",
		},
		DriverCredential: nil,
	}
	ctx := context.Background()

	require.NoError(t, drv.RunMigration(ctx))
	require.NoError(t, drv.EnsureSystemBootstrap(ctx))

	n, err := db.NewSelect().Model((*models.SystemUser)(nil)).Where("email = ?", bootstrapmeta.AdminEmail).Count(ctx)
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, 1)

	u, err := drv.GetSystemUserByEmail(ctx, bootstrapmeta.AdminEmail)
	require.NoError(t, err)
	require.NotNil(t, u)
	require.Equal(t, bootstrapmeta.AdminEmail, u.Email)
	require.NotEmpty(t, u.DefaultTeamID)
	require.NotEmpty(t, u.DefaultOrganizationID)

	proj, err := drv.GetProject(ctx, bootstrapmeta.StarterProjectID)
	require.NoError(t, err)
	require.NotNil(t, proj)
	require.Equal(t, bootstrapmeta.StarterProjectName, proj.Name)
	require.NotNil(t, proj.Driver)
	require.Equal(t, bootstrapmeta.StarterProjectID, proj.Driver.ProjectID)
	require.Equal(t, "coredb", proj.Driver.Engine)
	require.Equal(t, bootstrapmeta.OSSStarterSQLiteFile, proj.Driver.File)
	require.NotNil(t, proj.Settings)
	require.Equal(t, []string{"en"}, proj.Settings.Locals)

	var upCount int
	err = db.NewSelect().ColumnExpr("count(*)").Table("user_projects").
		Where("user_id = ? AND project_id = ?", u.ID, bootstrapmeta.StarterProjectID).
		Scan(ctx, &upCount)
	require.NoError(t, err)
	require.GreaterOrEqual(t, upCount, 1)

	var utCount int
	err = db.NewSelect().ColumnExpr("count(*)").Table("user_teams").
		Where("user_id = ?", u.ID).
		Scan(ctx, &utCount)
	require.NoError(t, err)
	require.GreaterOrEqual(t, utCount, 1)

	var uoCount int
	err = db.NewSelect().ColumnExpr("count(*)").Table("user_organizations").
		Where("user_id = ? AND role = ?", u.ID, "admin").
		Scan(ctx, &uoCount)
	require.NoError(t, err)
	require.GreaterOrEqual(t, uoCount, 1)

	var tpCount int
	err = db.NewSelect().ColumnExpr("count(*)").Table("team_projects").
		Where("project_id = ?", bootstrapmeta.StarterProjectID).
		Scan(ctx, &tpCount)
	require.NoError(t, err)
	require.GreaterOrEqual(t, tpCount, 1)

	require.NoError(t, drv.EnsureSystemBootstrap(ctx))
}

func TestGetProjectWithRolesAndPermission_NoProjectWithRolesTable(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := &SystemSQLDriver{
		ORM: db,
		Conf: &models.Config{
			SystemDatabaseEngine: "coredb",
		},
		DriverCredential: nil,
	}
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))
	require.NoError(t, drv.EnsureSystemBootstrap(ctx))

	u, err := drv.GetSystemUserByEmail(ctx, "admin@apito.io")
	require.NoError(t, err)
	list, err := drv.GetProjectWithRolesAndPermission(ctx, u.ID)
	require.NoError(t, err)
	require.NotEmpty(t, list)
	require.NotNil(t, list[0].Project)
	require.Equal(t, u.ID, list[0].User.ID)
}

func TestUpdateProject_persistsModelTypes(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := &SystemSQLDriver{
		ORM: db,
		Conf: &models.Config{
			SystemDatabaseEngine: "coredb",
		},
		DriverCredential: nil,
	}
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))
	require.NoError(t, drv.EnsureSystemBootstrap(ctx))

	pid := bootstrapmeta.StarterProjectID
	proj, err := drv.GetProject(ctx, pid)
	require.NoError(t, err)
	require.NotNil(t, proj)

	proj.Schema = &models.ProjectSchema{
		Models: []*models.ModelType{
			{Name: "author", SinglePage: true},
		},
	}
	require.NoError(t, drv.UpdateProject(ctx, proj, false))

	var n int
	err = db.NewSelect().ColumnExpr("count(*)").Table("model_types").
		Where("project_id = ? AND name = ?", pid, "author").
		Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	reloaded, err := drv.GetProject(ctx, pid)
	require.NoError(t, err)
	require.NotNil(t, reloaded.Schema)
	require.Len(t, reloaded.Schema.Models, 1)
	require.Equal(t, "author", reloaded.Schema.Models[0].Name)
	require.Equal(t, pid, reloaded.Schema.Models[0].ProjectID)
}

func TestUpdateProject_removesOrphanModelTypes(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := &SystemSQLDriver{
		ORM: db,
		Conf: &models.Config{
			SystemDatabaseEngine: "coredb",
		},
		DriverCredential: nil,
	}
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))
	require.NoError(t, drv.EnsureSystemBootstrap(ctx))

	pid := bootstrapmeta.StarterProjectID
	proj, err := drv.GetProject(ctx, pid)
	require.NoError(t, err)
	require.NotNil(t, proj)

	proj.Schema = &models.ProjectSchema{
		Models: []*models.ModelType{
			{Name: "author", SinglePage: true},
			{Name: "to_be_removed", SinglePage: false},
		},
	}
	require.NoError(t, drv.UpdateProject(ctx, proj, false))

	var nRemoved int
	err = db.NewSelect().ColumnExpr("count(*)").Table("model_types").
		Where("project_id = ? AND name = ?", pid, "to_be_removed").
		Scan(ctx, &nRemoved)
	require.NoError(t, err)
	require.Equal(t, 1, nRemoved)

	proj.Schema = &models.ProjectSchema{
		Models: []*models.ModelType{
			{Name: "author", SinglePage: true},
		},
	}
	require.NoError(t, drv.UpdateProject(ctx, proj, false))

	err = db.NewSelect().ColumnExpr("count(*)").Table("model_types").
		Where("project_id = ? AND name = ?", pid, "to_be_removed").
		Scan(ctx, &nRemoved)
	require.NoError(t, err)
	require.Equal(t, 0, nRemoved)

	var total int
	err = db.NewSelect().ColumnExpr("count(*)").Table("model_types").
		Where("project_id = ?", pid).
		Scan(ctx, &total)
	require.NoError(t, err)
	require.Equal(t, 1, total)

	reloaded, err := drv.GetProject(ctx, pid)
	require.NoError(t, err)
	require.NotNil(t, reloaded.Schema)
	require.Len(t, reloaded.Schema.Models, 1)
	require.Equal(t, "author", reloaded.Schema.Models[0].Name)
}

func TestUpsertModelType_updatesOneRowLeavesOtherUnchanged(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := &SystemSQLDriver{
		ORM: db,
		Conf: &models.Config{
			SystemDatabaseEngine: "coredb",
		},
		DriverCredential: nil,
	}
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))
	require.NoError(t, drv.EnsureSystemBootstrap(ctx))

	pid := bootstrapmeta.StarterProjectID
	proj, err := drv.GetProject(ctx, pid)
	require.NoError(t, err)
	proj.Schema = &models.ProjectSchema{
		Models: []*models.ModelType{
			{Name: "alpha", Description: "da"},
			{Name: "beta", Description: "db"},
		},
	}
	require.NoError(t, drv.UpdateProject(ctx, proj, false))

	reloaded, err := drv.GetProject(ctx, pid)
	require.NoError(t, err)
	var betaFull *models.ModelType
	for _, m := range reloaded.Schema.Models {
		if m != nil && m.Name == "beta" {
			betaFull = m
			break
		}
	}
	require.NotNil(t, betaFull)
	betaFull.Description = "db-new"
	require.NoError(t, drv.UpsertModelType(ctx, pid, betaFull))

	var da, descBeta string
	err = db.NewSelect().Column("description").Table("model_types").
		Where("project_id = ? AND name = ?", pid, "alpha").
		Scan(ctx, &da)
	require.NoError(t, err)
	require.Equal(t, "da", da)
	err = db.NewSelect().Column("description").Table("model_types").
		Where("project_id = ? AND name = ?", pid, "beta").
		Scan(ctx, &descBeta)
	require.NoError(t, err)
	require.Equal(t, "db-new", descBeta)
}

func TestTouchProjectUpdatedAt_changesProjectsTimestamp(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := &SystemSQLDriver{
		ORM: db,
		Conf: &models.Config{
			SystemDatabaseEngine: "coredb",
		},
		DriverCredential: nil,
	}
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))
	require.NoError(t, drv.EnsureSystemBootstrap(ctx))

	pid := bootstrapmeta.StarterProjectID
	proj, err := drv.GetProject(ctx, pid)
	require.NoError(t, err)
	before := proj.UpdatedAt
	time.Sleep(2 * time.Millisecond)
	require.NoError(t, drv.TouchProjectUpdatedAt(ctx, pid))
	proj2, err := drv.GetProject(ctx, pid)
	require.NoError(t, err)
	require.NotEqual(t, before, proj2.UpdatedAt)
}

func TestCreateProject_persistsSchemaModelsOnInsert(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := &SystemSQLDriver{
		ORM: db,
		Conf: &models.Config{
			SystemDatabaseEngine: "coredb",
		},
		DriverCredential: nil,
	}
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))
	require.NoError(t, drv.EnsureSystemBootstrap(ctx))

	u, err := drv.GetSystemUserByEmail(ctx, bootstrapmeta.AdminEmail)
	require.NoError(t, err)

	pid := "baadbeef-baad-baad-baad-baadbeef0001"
	proj := &models.Project{
		ID:   pid,
		Name: "SaaS create schema test",
		Driver: &models.DriverCredentials{
			ProjectID: pid,
			Engine:    "coredb",
			File:      "baadbeef_baad_baad_baad_baadbeef0001.sqlite",
		},
		Schema: &models.ProjectSchema{
			Models: []*models.ModelType{
				{
					Name: "tenant",
					Fields: []*models.FieldInfo{
						{Identifier: "name", FieldType: "text", InputType: "string", Serial: 1, Label: "Name"},
						{Identifier: "logo", FieldType: "media", InputType: "string", Serial: 2, Label: "Logo"},
					},
				},
			},
		},
	}
	created, err := drv.CreateProject(ctx, u.ID, proj)
	require.NoError(t, err)
	require.Equal(t, pid, created.ID)

	var n int
	err = db.NewSelect().ColumnExpr("count(*)").Table("model_types").
		Where("project_id = ? AND name = ?", pid, "tenant").
		Scan(ctx, &n)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	reloaded, err := drv.GetProject(ctx, pid)
	require.NoError(t, err)
	require.NotNil(t, reloaded.Schema)
	require.Len(t, reloaded.Schema.Models, 1)
	require.Equal(t, "tenant", reloaded.Schema.Models[0].Name)
	require.NotNil(t, reloaded.Settings)
	require.Equal(t, []string{"en"}, reloaded.Settings.Locals)

	var settingsCount int
	err = db.NewSelect().ColumnExpr("count(*)").Table("project_settings").
		Where("project_id = ?", pid).
		Scan(ctx, &settingsCount)
	require.NoError(t, err)
	require.Equal(t, 1, settingsCount)
}

func TestEnsureBootstrapOrgTeamProjectWithStarterCreate_InvokesFnOnce(t *testing.T) {
	sqldb, err := sql.Open(sqliteshim.ShimName, "file::memory:?cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	drv := &SystemSQLDriver{
		ORM: db,
		Conf: &models.Config{
			SystemDatabaseEngine: "coredb",
		},
		DriverCredential: nil,
	}
	ctx := context.Background()
	require.NoError(t, drv.RunMigration(ctx))
	require.NoError(t, drv.ensureBootstrapAdmin(ctx))

	var fnCalls int
	err = drv.EnsureBootstrapOrgTeamProjectWithStarterCreate(ctx, func(ctx context.Context, userID string, proj *models.Project) error {
		fnCalls++
		_, err := drv.CreateProject(ctx, userID, proj)
		return err
	})
	require.NoError(t, err)
	require.Equal(t, 1, fnCalls)

	err = drv.EnsureBootstrapOrgTeamProjectWithStarterCreate(ctx, func(ctx context.Context, userID string, proj *models.Project) error {
		fnCalls++
		return errors.New("starter create should not run when org defaults exist")
	})
	require.NoError(t, err)
	require.Equal(t, 1, fnCalls)
}

package projectauthusers_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/apito-io/engine/database/project/projectauthusers"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "github.com/uptrace/bun/driver/sqliteshim"
)

func TestSQLStoreAuthUserCRUD(t *testing.T) {
	sqldb, err := sql.Open("sqlite", "file:authusers?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })

	db := bun.NewDB(sqldb, sqlitedialect.New())
	store := &projectauthusers.SQLStore{DB: db}
	ctx := context.Background()

	require.NoError(t, store.EnsureUsersTable(ctx))

	row := &models.ProjectAuthUser{
		ID:       "u1",
		Username: "alice",
		Email:    "alice@example.com",
		Phone:    "+15551212",
		Secret:   "hash",
		Role:     "none",
		Provider: models.UserProviderLocal,
		Status:   models.UserStatusActive,
	}
	created, err := store.CreateProjectAuthUser(ctx, row)
	require.NoError(t, err)
	require.Equal(t, "u1", created.ID)

	got, err := store.GetProjectAuthUser(ctx, "u1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "alice@example.com", got.Email)

	byEmail, err := store.ListProjectAuthUsersByEmail(ctx, "", "alice@example.com")
	require.NoError(t, err)
	require.Len(t, byEmail, 1)

	rows, count, err := store.SearchProjectAuthUsers(ctx, "", 10, 0)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Len(t, rows, 1)

	got.Role = "admin"
	require.NoError(t, store.UpdateProjectAuthUser(ctx, got))
	refreshed, err := store.GetProjectAuthUser(ctx, "u1")
	require.NoError(t, err)
	require.Equal(t, "admin", refreshed.Role)

	require.NoError(t, store.DeleteProjectAuthUser(ctx, "u1"))
	missing, err := store.GetProjectAuthUser(ctx, "u1")
	require.NoError(t, err)
	require.Nil(t, missing)
}

func TestSQLStoreCountProjectAuthUsersByRole(t *testing.T) {
	sqldb, err := sql.Open("sqlite", "file:authuserscount?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })

	db := bun.NewDB(sqldb, sqlitedialect.New())
	store := &projectauthusers.SQLStore{DB: db}
	ctx := context.Background()

	require.NoError(t, store.EnsureUsersTable(ctx))

	for _, spec := range []struct {
		id, role string
	}{
		{"u1", "public"},
		{"u2", "public"},
		{"u3", "receptionist"},
	} {
		_, err := store.CreateProjectAuthUser(ctx, &models.ProjectAuthUser{
			ID:       spec.id,
			Username: spec.id,
			Email:    spec.id + "@example.com",
			Secret:   "hash",
			Role:     spec.role,
			Provider: models.UserProviderLocal,
			Status:   models.UserStatusActive,
		})
		require.NoError(t, err)
	}

	counts, err := store.CountProjectAuthUsersByRole(ctx, "")
	require.NoError(t, err)
	require.Equal(t, 2, counts["public"])
	require.Equal(t, 1, counts["receptionist"])
}

func TestSQLStoreUniqueEmailPhoneGoogleSub(t *testing.T) {
	sqldb, err := sql.Open("sqlite", "file:authusersunique?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })

	db := bun.NewDB(sqldb, sqlitedialect.New())
	store := &projectauthusers.SQLStore{DB: db}
	ctx := context.Background()

	require.NoError(t, store.EnsureUsersTable(ctx))

	base := &models.ProjectAuthUser{
		ID:       "u1",
		Username: "alice",
		Email:    "alice@example.com",
		Phone:    "+15551111",
		Secret:   "hash",
		Role:     "none",
		Provider: models.UserProviderLocal,
		Status:   models.UserStatusActive,
	}
	_, err = store.CreateProjectAuthUser(ctx, base)
	require.NoError(t, err)

	dupEmail := &models.ProjectAuthUser{
		ID:       "u2",
		Username: "bob",
		Email:    "alice@example.com",
		Secret:   "hash",
		Role:     "none",
		Provider: models.UserProviderLocal,
		Status:   models.UserStatusActive,
	}
	_, err = store.CreateProjectAuthUser(ctx, dupEmail)
	require.Error(t, err)
	require.Contains(t, err.Error(), "email already exists")

	dupPhone := &models.ProjectAuthUser{
		ID:       "u3",
		Username: "carol",
		Email:    "carol@example.com",
		Phone:    "+15551111",
		Secret:   "hash",
		Role:     "none",
		Provider: models.UserProviderLocal,
		Status:   models.UserStatusActive,
	}
	_, err = store.CreateProjectAuthUser(ctx, dupPhone)
	require.Error(t, err)
	require.Contains(t, err.Error(), "phone already exists")

	withGoogle := &models.ProjectAuthUser{
		ID:        "u4",
		Username:  "dave",
		Email:     "dave@example.com",
		GoogleSub: "google-sub-1",
		Secret:    "hash",
		Role:      "none",
		Provider:  models.UserProviderGoogle,
		Status:    models.UserStatusActive,
	}
	_, err = store.CreateProjectAuthUser(ctx, withGoogle)
	require.NoError(t, err)

	dupGoogle := &models.ProjectAuthUser{
		ID:        "u5",
		Username:  "eve",
		Email:     "eve@example.com",
		GoogleSub: "google-sub-1",
		Secret:    "hash",
		Role:      "none",
		Provider:  models.UserProviderGoogle,
		Status:    models.UserStatusActive,
	}
	_, err = store.CreateProjectAuthUser(ctx, dupGoogle)
	require.Error(t, err)
	require.Contains(t, err.Error(), "google account already linked")
}

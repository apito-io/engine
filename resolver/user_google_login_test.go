package resolver

import (
	"context"
	"database/sql"
	"testing"

	"gitlab.com/apito.io/open_driver/project/projectauthusers"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	_ "github.com/uptrace/bun/driver/sqliteshim"
	"google.golang.org/api/idtoken"
)

func newMemProjectUserService(t *testing.T) (*ProjectUserService, *projectauthusers.SQLStore) {
	t.Helper()
	sqldb, err := sql.Open("sqlite", "file:googlelogin"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqldb.Close() })
	db := bun.NewDB(sqldb, sqlitedialect.New())
	store := &projectauthusers.SQLStore{DB: db}
	ctx := context.Background()
	require.NoError(t, store.EnsureUsersTable(ctx))
	return &ProjectUserService{
		ctx:       ctx,
		projectID: "proj1",
		store:     store,
	}, store
}

func TestGoogleEmailVerified(t *testing.T) {
	require.True(t, GoogleEmailVerified(&idtoken.Payload{Claims: map[string]interface{}{"email_verified": true}}))
	require.True(t, GoogleEmailVerified(&idtoken.Payload{Claims: map[string]interface{}{"email_verified": "true"}}))
	require.False(t, GoogleEmailVerified(&idtoken.Payload{Claims: map[string]interface{}{"email_verified": false}}))
	require.False(t, GoogleEmailVerified(nil))
}

func TestResolveUserForGoogleLogin_AutoCreate(t *testing.T) {
	svc, _ := newMemProjectUserService(t)
	user, err := svc.ResolveUserForGoogleLogin("sub-new", "new@example.com", true, "", "", func() (*models.User, error) {
		return svc.CreateUserRecord(&models.User{
			ID:       "u-new",
			Username: "u_new",
			Email:    "new@example.com",
			GoogleSub: "sub-new",
			Role:     "none",
			Provider: models.UserProviderGoogle,
			Status:   models.UserStatusActive,
		}, "")
	})
	require.NoError(t, err)
	require.Equal(t, "u-new", user.ID)
	require.Equal(t, "sub-new", user.GoogleSub)
}

func TestResolveUserForGoogleLogin_LinkExistingEmail(t *testing.T) {
	svc, store := newMemProjectUserService(t)
	ctx := context.Background()
	_, err := store.CreateProjectAuthUser(ctx, &models.ProjectAuthUser{
		ID:       "u-existing",
		Username: "alice",
		Email:    "alice@example.com",
		Secret:   "hash",
		Role:     "none",
		Provider: models.UserProviderLocal,
		Status:   models.UserStatusActive,
	})
	require.NoError(t, err)

	user, err := svc.ResolveUserForGoogleLogin("sub-link", "alice@example.com", true, "", "", nil)
	require.NoError(t, err)
	require.Equal(t, "u-existing", user.ID)
	require.Equal(t, "sub-link", user.GoogleSub)

	row, err := store.GetProjectAuthUser(ctx, "u-existing")
	require.NoError(t, err)
	require.Equal(t, "sub-link", row.GoogleSub)
	require.Equal(t, "hash", row.Secret)
}

func TestResolveUserForGoogleLogin_RejectMissingEmailOnCreate(t *testing.T) {
	svc, _ := newMemProjectUserService(t)
	created := false
	_, err := svc.ResolveUserForGoogleLogin("sub-no-email", "", true, "", "", func() (*models.User, error) {
		created = true
		return &models.User{ID: "should-not"}, nil
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "google token missing email")
	require.False(t, created)
}

func TestResolveUserForGoogleLogin_BackfillEmailOnGoogleSubHit(t *testing.T) {
	svc, store := newMemProjectUserService(t)
	ctx := context.Background()
	_, err := store.CreateProjectAuthUser(ctx, &models.ProjectAuthUser{
		ID:        "u-orphan",
		Username:  "u_orphan",
		Email:     "",
		GoogleSub: "sub-orphan",
		Role:      "vendor",
		Provider:  models.UserProviderGoogle,
		Status:    models.UserStatusActive,
	})
	require.NoError(t, err)

	user, err := svc.ResolveUserForGoogleLogin("sub-orphan", "recovered@example.com", true, "", "", nil)
	require.NoError(t, err)
	require.Equal(t, "u-orphan", user.ID)
	require.Equal(t, "recovered@example.com", user.Email)

	row, err := store.GetProjectAuthUser(ctx, "u-orphan")
	require.NoError(t, err)
	require.Equal(t, "recovered@example.com", row.Email)
}

func TestResolveUserForGoogleLogin_RejectConflictingGoogleSub(t *testing.T) {
	svc, store := newMemProjectUserService(t)
	ctx := context.Background()
	_, err := store.CreateProjectAuthUser(ctx, &models.ProjectAuthUser{
		ID:        "u1",
		Username:  "bob",
		Email:     "bob@example.com",
		GoogleSub: "sub-other",
		Role:      "none",
		Provider:  models.UserProviderLocal,
		Status:    models.UserStatusActive,
	})
	require.NoError(t, err)

	_, err = svc.ResolveUserForGoogleLogin("sub-new", "bob@example.com", true, "", "", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "google account already linked")
}

func TestResolveUserForGoogleLogin_RejectMultipleEmailMatches(t *testing.T) {
	dupEmail := "dup@example.com"
	svc := &ProjectUserService{
		ctx:       context.Background(),
		projectID: "proj1",
		sys: &multiEmailSystemDriver{
			byEmail: map[string][]*models.User{
				dupEmail: {
					{ID: "u1", Email: dupEmail, Status: models.UserStatusActive},
					{ID: "u2", Email: dupEmail, Status: models.UserStatusActive},
				},
			},
		},
	}

	_, err := svc.ResolveUserForGoogleLogin("sub-dup", dupEmail, true, "", "", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "multiple users matched this email")
}

type multiEmailSystemDriver struct {
	trackingSystemDriver
	byEmail map[string][]*models.User
}

func (d *multiEmailSystemDriver) ListUsersByEmail(_ context.Context, _ string, email string) ([]*models.User, error) {
	if d == nil || d.byEmail == nil {
		return nil, nil
	}
	return d.byEmail[email], nil
}

func (d *multiEmailSystemDriver) ListUsersByGoogleSub(context.Context, string, string) ([]*models.User, error) {
	return nil, nil
}

func TestAssertUserEmailUnique(t *testing.T) {
	svc, store := newMemProjectUserService(t)
	ctx := context.Background()
	_, err := store.CreateProjectAuthUser(ctx, &models.ProjectAuthUser{
		ID:       "u1",
		Username: "u1",
		Email:    "taken@example.com",
		Role:     "none",
		Provider: models.UserProviderLocal,
		Status:   models.UserStatusActive,
	})
	require.NoError(t, err)

	require.NoError(t, svc.assertUserEmailUnique("", "free@example.com", ""))
	err = svc.assertUserEmailUnique("", "taken@example.com", "")
	require.Error(t, err)
	require.Equal(t, UserDuplicateExistsMessage("email", false), err.Error())
	require.NoError(t, svc.assertUserEmailUnique("", "taken@example.com", "u1"))
}

func TestAssertUserPhoneUnique(t *testing.T) {
	svc, store := newMemProjectUserService(t)
	ctx := context.Background()
	phone := models.NormalizeUserPhoneKey("+15551234567")
	_, err := store.CreateProjectAuthUser(ctx, &models.ProjectAuthUser{
		ID:       "u1",
		Username: "u1",
		Email:    "u1@example.com",
		Phone:    phone,
		Role:     "none",
		Provider: models.UserProviderLocal,
		Status:   models.UserStatusActive,
	})
	require.NoError(t, err)

	err = svc.assertUserPhoneUnique("", phone, "")
	require.Error(t, err)
	require.Equal(t, UserDuplicateExistsMessage("phone", false), err.Error())
}

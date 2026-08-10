package resolver

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

type countingSystemDriver struct {
	trackingSystemDriver
	counts map[string]int
}

func (d *countingSystemDriver) CountProjectUsersByRole(context.Context, string) (map[string]int, error) {
	return d.counts, nil
}

func TestProjectUserServiceCountUsersByRoleFallback(t *testing.T) {
	svc := &ProjectUserService{
		ctx:       context.Background(),
		projectID: "p1",
		sys: &countingSystemDriver{
			counts: map[string]int{"public": 3},
		},
	}
	counts, err := svc.CountUsersByRole()
	if err != nil {
		t.Fatalf("CountUsersByRole: %v", err)
	}
	if counts["public"] != 3 {
		t.Fatalf("expected 3 public users, got %#v", counts)
	}
}

type failingAuthUserStore struct {
	err error
}

func (f *failingAuthUserStore) EnsureUsersTable(context.Context) error { return nil }
func (f *failingAuthUserStore) CreateProjectAuthUser(context.Context, *models.ProjectAuthUser) (*models.ProjectAuthUser, error) {
	return nil, nil
}
func (f *failingAuthUserStore) GetProjectAuthUser(context.Context, string) (*models.ProjectAuthUser, error) {
	return nil, nil
}
func (f *failingAuthUserStore) GetProjectAuthUserByUsername(context.Context, string) (*models.ProjectAuthUser, error) {
	return nil, nil
}
func (f *failingAuthUserStore) ListProjectAuthUsersByEmail(context.Context, string, string) ([]*models.ProjectAuthUser, error) {
	return nil, nil
}
func (f *failingAuthUserStore) ListProjectAuthUsersByPhone(context.Context, string, string) ([]*models.ProjectAuthUser, error) {
	return nil, nil
}
func (f *failingAuthUserStore) ListProjectAuthUsersByGoogleSub(context.Context, string, string) ([]*models.ProjectAuthUser, error) {
	return nil, nil
}
func (f *failingAuthUserStore) ListProjectAuthUsersByOAuthSub(context.Context, string, string, string) ([]*models.ProjectAuthUser, error) {
	return nil, nil
}
func (f *failingAuthUserStore) SearchProjectAuthUsers(context.Context, string, string, int, int) ([]*models.ProjectAuthUser, int, error) {
	return nil, 0, nil
}
func (f *failingAuthUserStore) CountProjectAuthUsersByRole(context.Context, string) (map[string]int, error) {
	return nil, f.err
}
func (f *failingAuthUserStore) UpdateProjectAuthUser(context.Context, *models.ProjectAuthUser) error {
	return nil
}
func (f *failingAuthUserStore) DeleteProjectAuthUser(context.Context, string) error { return nil }

func TestProjectUserServiceCountUsersByRoleStoreErrorFallback(t *testing.T) {
	svc := &ProjectUserService{
		ctx:       context.Background(),
		projectID: "p1",
		store:     &failingAuthUserStore{err: errors.New("tenant db unavailable")},
		sys: &countingSystemDriver{
			counts: map[string]int{"admin": 2},
		},
	}
	counts, err := svc.CountUsersByRole()
	if err != nil {
		t.Fatalf("CountUsersByRole: %v", err)
	}
	if counts["admin"] != 2 {
		t.Fatalf("expected system fallback counts, got %#v", counts)
	}
}

func TestDuplicateRoleClonePreservesPermissionsWithoutAdmin(t *testing.T) {
	src := &models.Role{
		IsAdmin: true,
		APIPermissions: map[string]*models.APIPermission{
			"employee": {Read: "all", Create: "none", Update: "none", Delete: "none"},
		},
	}
	copy := utility.CloneRole(src)
	if copy.IsAdmin || copy.SystemGenerated {
		t.Fatal("duplicate must not grant admin/system flags")
	}
	if copy.APIPermissions["employee"].Read != "all" {
		t.Fatal("permissions must be copied")
	}
}

func TestApplyRoleUpsertIsAdminRejectsGrant(t *testing.T) {
	role := &models.Role{IsAdmin: false}
	err := applyRoleUpsertIsAdmin(role, map[string]interface{}{"is_admin": true})
	if err == nil {
		t.Fatal("expected rejection when granting is_admin")
	}
	if !strings.Contains(err.Error(), "cannot grant is_admin") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestApplyRoleUpsertIsAdminClearsNonSystem(t *testing.T) {
	role := &models.Role{IsAdmin: true}
	if err := applyRoleUpsertIsAdmin(role, map[string]interface{}{"is_admin": false}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role.IsAdmin {
		t.Fatal("expected IsAdmin cleared")
	}
}

func TestApplyRoleUpsertIsAdminKeepsSystemGenerated(t *testing.T) {
	role := &models.Role{IsAdmin: true, SystemGenerated: true}
	if err := applyRoleUpsertIsAdmin(role, map[string]interface{}{"is_admin": false}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !role.IsAdmin {
		t.Fatal("system_generated admin flag must not be cleared")
	}
}

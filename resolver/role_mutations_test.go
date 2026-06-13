package resolver

import (
	"context"
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

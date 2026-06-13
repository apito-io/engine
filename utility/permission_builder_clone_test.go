package utility

import (
	"testing"

	"github.com/apito-io/engine/models"
)

func TestCloneRoleCopiesPermissionsAndClearsFlags(t *testing.T) {
	src := &models.Role{
		SystemGenerated:           true,
		IsAdmin:                   true,
		LogicExecutions:           []string{"fn1"},
		AdministrativePermissions: []string{"all"},
		APIPermissions: map[string]*models.APIPermission{
			"food_order": {
				Read:   "all",
				Create: "none",
				Update: "own",
				Delete: "none",
			},
		},
	}

	dst := CloneRole(src)
	if dst == nil {
		t.Fatal("expected clone")
	}
	if dst.SystemGenerated || dst.IsAdmin {
		t.Fatal("copied role must not be admin or system generated")
	}
	if len(dst.LogicExecutions) != 1 || dst.LogicExecutions[0] != "fn1" {
		t.Fatalf("logic executions not copied: %#v", dst.LogicExecutions)
	}
	if len(dst.AdministrativePermissions) != 1 || dst.AdministrativePermissions[0] != "all" {
		t.Fatalf("administrative permissions not copied: %#v", dst.AdministrativePermissions)
	}
	ap, ok := dst.APIPermissions["food_order"]
	if !ok || ap == nil {
		t.Fatal("api permissions not copied")
	}
	if ap.Read != "all" || ap.Update != "own" {
		t.Fatalf("unexpected permission values: %#v", ap)
	}

	src.APIPermissions["food_order"].Read = "none"
	if dst.APIPermissions["food_order"].Read != "all" {
		t.Fatal("clone must deep-copy api permission values")
	}
}

func TestCloneRoleNilSource(t *testing.T) {
	dst := CloneRole(nil)
	if dst == nil {
		t.Fatal("expected non-nil empty role")
	}
	if dst.SystemGenerated || dst.IsAdmin {
		t.Fatal("empty clone must not be admin or system generated")
	}
}

package utility

import (
	"testing"

	"github.com/apito-io/engine/models"
)

func TestMoveAPIPermissionKey_renamesAndDropsOld(t *testing.T) {
	perms := map[string]*models.APIPermission{
		"tag": {Read: "all", Create: "all", Update: "all", Delete: "none"},
	}
	MoveAPIPermissionKey(perms, "tag", "variant")
	if _, ok := perms["tag"]; ok {
		t.Fatal("old key tag must be removed")
	}
	got := perms["variant"]
	if got == nil || got.Read != "all" || got.Create != "all" {
		t.Fatalf("variant grant = %#v", got)
	}
}

func TestMoveAPIPermissionKey_keepsExistingNewKey(t *testing.T) {
	perms := map[string]*models.APIPermission{
		"tag":     {Read: "all", Create: "none", Update: "none", Delete: "none"},
		"variant": {Read: "all", Create: "all", Update: "all", Delete: "none"},
	}
	MoveAPIPermissionKey(perms, "tag", "variant")
	if _, ok := perms["tag"]; ok {
		t.Fatal("old key tag must be removed")
	}
	if perms["variant"].Create != "all" {
		t.Fatalf("must not overwrite existing variant grant: %#v", perms["variant"])
	}
}

func TestRewriteProjectPermissionKeys_rolesAndPlans(t *testing.T) {
	project := &models.Project{
		Roles: map[string]*models.Role{
			"manager": {APIPermissions: map[string]*models.APIPermission{
				"tag": {Read: "all", Create: "all", Update: "all", Delete: "none"},
			}},
		},
		Plans: map[string]*models.Plan{
			"ultra": {APIPermissions: map[string]*models.APIPermission{
				"tag": {Read: "all", Create: "all", Update: "all", Delete: "none"},
			}},
		},
	}
	RewriteProjectPermissionKeys(project, "tag", "variant")
	if project.Roles["manager"].APIPermissions["variant"] == nil {
		t.Fatal("manager must grant variant after rename")
	}
	if project.Plans["ultra"].APIPermissions["variant"] == nil {
		t.Fatal("plan must grant variant after rename")
	}
}

func TestValidateScopeRejectsCustomLogic(t *testing.T) {
	if _, ok := ValidateScope("custom_logic"); ok {
		t.Fatal("custom_logic must be rejected by ValidateScope")
	}
	if _, ok := ValidateReadScope("custom_logic"); ok {
		t.Fatal("custom_logic must be rejected by ValidateReadScope")
	}
	if _, ok := ValidateCreateScope("custom_logic"); ok {
		t.Fatal("custom_logic must be rejected by ValidateCreateScope")
	}
}

func TestValidateCreateScopeRejectsOwn(t *testing.T) {
	if _, ok := ValidateCreateScope("own"); ok {
		t.Fatal("create must reject own")
	}
	for _, s := range []string{"none", "all", "auth"} {
		if _, ok := ValidateCreateScope(s); !ok {
			t.Fatalf("create must accept %q", s)
		}
	}
}

func TestEffectivePermissionDemoReadOnly(t *testing.T) {
	role := &models.Role{ID: "demo", SystemGenerated: true}
	perm := EffectivePermission(role, "food_order")
	if perm.Read != "all" || perm.Create != "none" || perm.Update != "none" || perm.Delete != "none" {
		t.Fatalf("demo must be read-only: %#v", perm)
	}
}

func TestEffectivePermissionSystemGeneratedTeamWithoutPermissionsIsNone(t *testing.T) {
	role := &models.Role{ID: "team", SystemGenerated: true}
	perm := EffectivePermission(role, "food_order")
	if perm.Read != "none" || perm.Create != "none" || perm.Update != "none" || perm.Delete != "none" {
		t.Fatalf("system team without permissions must be all none: %#v", perm)
	}
}

func TestEffectivePermissionAdminAll(t *testing.T) {
	for _, role := range []*models.Role{
		{ID: "admin"},
		{ID: "owner"},
		{ID: "editor", IsAdmin: true},
	} {
		perm := EffectivePermission(role, "food_order")
		if perm.Read != "all" || perm.Create != "all" || perm.Update != "all" || perm.Delete != "all" {
			t.Fatalf("admin bypass failed for %#v → %#v", role, perm)
		}
		if !RoleBypassesDataACL(role) {
			t.Fatalf("RoleBypassesDataACL should be true for %#v", role)
		}
	}
}

func TestMigrateMapsCustomLogicToNone(t *testing.T) {
	ap := &models.APIPermission{
		Read: "custom_logic", Create: "all", Update: "custom_logic", Delete: "own",
	}
	if !MigrateAPIPermissionScopes(ap) {
		t.Fatal("expected change")
	}
	if ap.Read != "none" || ap.Update != "none" || ap.Create != "all" || ap.Delete != "own" {
		t.Fatalf("unexpected after migrate: %#v", ap)
	}
	if NormalizeLegacyScope("custom_logic") != "none" {
		t.Fatal("NormalizeLegacyScope should map custom_logic to none")
	}

	role := &models.Role{
		APIPermissions: map[string]*models.APIPermission{
			"food": {Read: "custom_logic", Create: "custom_logic", Update: "all", Delete: "custom_logic"},
		},
	}
	n := MigrateRoleAPIPermissions(role)
	if n != 3 {
		t.Fatalf("expected 3 fields migrated, got %d", n)
	}
}

func TestAuthorizeModelReadAuthWithoutProjectUserFails(t *testing.T) {
	role := &models.Role{
		ID:            "staff",
		IsProjectUser: false,
		APIPermissions: map[string]*models.APIPermission{
			"food_order": {Read: "auth", Create: "none", Update: "none", Delete: "none"},
		},
	}
	if err := AuthorizeModelRead(role, "food_order"); err == nil {
		t.Fatal("auth read without IsProjectUser must fail")
	}
	role.IsProjectUser = true
	if err := AuthorizeModelRead(role, "food_order"); err != nil {
		t.Fatalf("auth read with IsProjectUser must pass: %v", err)
	}
}

func TestBuildCRUDPermissionsUsesEffectivePermission(t *testing.T) {
	role := &models.Role{ID: "demo", SystemGenerated: true}
	perm, err := BuildCRUDPermissions("x", role)
	if err != nil {
		t.Fatal(err)
	}
	if perm.Create != "none" {
		t.Fatalf("BuildCRUDPermissions must not grant create to demo: %#v", perm)
	}
	if _, err := BuildCRUDPermissions("x", nil); err == nil {
		t.Fatal("nil role must error")
	}
}

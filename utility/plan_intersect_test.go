package utility

import (
	"testing"

	"github.com/apito-io/engine/models"
)

func TestMinScopeLattice(t *testing.T) {
	if got := MinScope(OpRead, "all", "own"); got != "own" {
		t.Fatalf("min(all,own)=%s want own", got)
	}
	if got := MinScope(OpCreate, "all", "own"); got != "none" {
		t.Fatalf("create min with own → none, got %s", got)
	}
	if got := MinScope(OpUpdate, "auth", "all"); got != "auth" {
		t.Fatalf("min(auth,all)=%s want auth", got)
	}
}

func TestIntersectAPIPermissionReadGrace(t *testing.T) {
	role := &models.APIPermission{Read: "all", Create: "all", Update: "own", Delete: "none"}
	plan := &models.APIPermission{Read: "none", Create: "none", Update: "all", Delete: "all"}
	out, grace := IntersectAPIPermission(role, plan)
	if !grace {
		t.Fatal("expected read grace when plan.read=none")
	}
	if out.Read != "all" {
		t.Fatalf("read grace should keep role.read=all, got %s", out.Read)
	}
	if out.Create != "none" || out.Update != "own" || out.Delete != "none" {
		t.Fatalf("mutations must use min: %#v", out)
	}
}

func TestIntersectRoleWithPlanClampsAdmin(t *testing.T) {
	role := &models.Role{
		ID:      "admin",
		IsAdmin: true,
		APIPermissions: map[string]*models.APIPermission{
			"student": {Read: "all", Create: "all", Update: "all", Delete: "all"},
		},
	}
	plan := &models.Plan{
		ID: "free",
		APIPermissions: map[string]*models.APIPermission{
			"student": {Read: "all", Create: "none", Update: "none", Delete: "none"},
		},
	}
	out := IntersectRoleWithPlan(role, plan)
	if out.IsAdmin {
		t.Fatal("IsAdmin must be cleared under a plan")
	}
	if !out.PlanClamped {
		t.Fatal("PlanClamped must be set")
	}
	if RoleBypassesDataACL(out) {
		t.Fatal("admin id must not bypass ACL when PlanClamped")
	}
	perm := EffectivePermission(out, "student")
	if perm.Create != "none" || perm.Read != "all" {
		t.Fatalf("expected read all / create none, got %#v", perm)
	}
	// Original role must be untouched.
	if !role.IsAdmin || role.PlanClamped {
		t.Fatal("source role must not be mutated")
	}
}

func TestIntersectRoleWithPlanNilIsPassthroughClone(t *testing.T) {
	role := &models.Role{ID: "teacher", IsAdmin: false, IsProjectUser: true}
	out := IntersectRoleWithPlan(role, nil)
	if out == role {
		t.Fatal("must clone")
	}
	if out.ID != "teacher" || out.PlanClamped {
		t.Fatalf("nil plan should not clamp: %#v", out)
	}
}

func TestCheckPlanRecordsQuotaBlocksCreateOnlySemantics(t *testing.T) {
	plan := &models.Plan{Quotas: map[string]int{"max_records.student": 2}}
	if err := CheckPlanRecordsQuota(plan, "student", 1); err != nil {
		t.Fatal("under limit should pass")
	}
	if err := CheckPlanRecordsQuota(plan, "student", 2); err == nil {
		t.Fatal("at limit should block")
	}
	if err := CheckPlanRecordsQuota(plan, "student", 5); err == nil {
		t.Fatal("over limit should block")
	}
	if err := CheckPlanRecordsQuota(nil, "student", 100); err != nil {
		t.Fatal("nil plan unlimited")
	}
}

func TestComputeReadGraceByModel(t *testing.T) {
	role := &models.Role{
		APIPermissions: map[string]*models.APIPermission{
			"ledger": {Read: "all", Create: "all", Update: "all", Delete: "all"},
		},
	}
	plan := &models.Plan{
		APIPermissions: map[string]*models.APIPermission{
			"ledger": {Read: "none", Create: "none", Update: "none", Delete: "none"},
		},
	}
	grace := ComputeReadGraceByModel(role, plan)
	if !grace["ledger"] {
		t.Fatal("expected grace on ledger")
	}
}

func TestPlanAllowsLogic(t *testing.T) {
	if !PlanAllowsLogic(nil, "anything") {
		t.Fatal("nil plan allows all")
	}
	p := &models.Plan{LogicExecutions: []string{"*"}}
	if !PlanAllowsLogic(p, "fn") {
		t.Fatal("* allows all")
	}
	p.LogicExecutions = []string{"export_marks"}
	if PlanAllowsLogic(p, "other") {
		t.Fatal("should deny other")
	}
	if !PlanAllowsLogic(p, "export_marks") {
		t.Fatal("should allow listed fn")
	}
}

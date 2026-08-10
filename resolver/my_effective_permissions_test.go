package resolver

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/tailor-platform/graphql"
)

func TestMyEffectivePermissionsResolverFn_returnsManagerSnapshot(t *testing.T) {
	s := &GraphQLServer{}
	role := &models.Role{
		ID:            "manager",
		IsProjectUser: true,
		PlanClamped:   true,
		APIPermissions: map[string]*models.APIPermission{
			"student": {Read: "all", Create: "all", Update: "all", Delete: "none"},
			"exam":    {Read: "all", Create: "all", Update: "all", Delete: "none"},
		},
	}
	cache := &models.ApplicationCache{
		Project: &models.Project{
			ID: "protiva_xtg4d",
			Roles: map[string]*models.Role{
				"manager": {
					ID: "manager",
					APIPermissions: map[string]*models.APIPermission{
						"student": {Read: "all", Create: "all", Update: "all", Delete: "none"},
						"exam":    {Read: "all", Create: "all", Update: "all", Delete: "none"},
					},
				},
			},
			Plans: map[string]*models.Plan{
				"free": models.DefaultSeededPlans()["free"],
			},
		},
		Param: &models.CommonSystemParams{
			ProjectID:  "protiva_xtg4d",
			UserID:     "user_1",
			Role:       role,
			Plan:       "free",
			ActivePlan: models.DefaultSeededPlans()["free"],
		},
	}
	ctx := utility.WithApplicationCache(context.Background(), cache)
	out, err := s.MyEffectivePermissionsResolverFn(graphql.ResolveParams{Context: ctx})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("got %T", out)
	}
	if m["role_id"] != "manager" {
		t.Fatalf("role_id=%v", m["role_id"])
	}
	if m["plan_slug"] != "free" {
		t.Fatalf("plan_slug=%v", m["plan_slug"])
	}
	if m["plan_clamped"] != true {
		t.Fatalf("plan_clamped=%v", m["plan_clamped"])
	}
	perms, ok := m["api_permissions"].(map[string]interface{})
	if !ok || len(perms) < 2 {
		t.Fatalf("api_permissions=%#v", m["api_permissions"])
	}
	student, ok := perms["student"].(map[string]interface{})
	if !ok || student["read"] != "all" || student["delete"] != "none" {
		t.Fatalf("student=%#v", student)
	}
}

func TestMyEffectivePermissionsResolverFn_rejectsMissingCache(t *testing.T) {
	s := &GraphQLServer{}
	_, err := s.MyEffectivePermissionsResolverFn(graphql.ResolveParams{
		Context: context.Background(),
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

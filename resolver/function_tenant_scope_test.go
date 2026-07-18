package resolver

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
)

func TestApplyFunctionTenantScope_NoHookUsesExplicit(t *testing.T) {
	s := &GraphQLServer{Cfg: &models.Config{}}
	cache := &models.ApplicationCache{
		Param: &models.CommonSystemParams{Ext: map[string]interface{}{}},
		Ctx:   context.Background(),
	}
	tid, err := s.ApplyFunctionTenantScope(context.Background(), cache, models.FunctionTenantScopeDraftTest, "tenant-abc")
	if err != nil {
		t.Fatal(err)
	}
	if tid != "tenant-abc" {
		t.Fatalf("got %q", tid)
	}
}

func TestApplyFunctionTenantScope_NoHookFallsBackToExt(t *testing.T) {
	s := &GraphQLServer{Cfg: &models.Config{}}
	cache := &models.ApplicationCache{
		Param: &models.CommonSystemParams{
			Ext: map[string]interface{}{"tenant_id": "from-ext"},
		},
		Ctx: context.Background(),
	}
	tid, err := s.ApplyFunctionTenantScope(context.Background(), cache, models.FunctionTenantScopeLive, "")
	if err != nil {
		t.Fatal(err)
	}
	if tid != "from-ext" {
		t.Fatalf("got %q", tid)
	}
}

func TestApplyFunctionTenantScope_HookInjects(t *testing.T) {
	s := &GraphQLServer{
		Cfg: &models.Config{
			FunctionTenantScopeHook: func(
				ctx context.Context,
				cache *models.ApplicationCache,
				mode models.FunctionTenantScopeMode,
				explicitTenantID string,
			) (string, error) {
				if mode != models.FunctionTenantScopeDraftTest {
					t.Fatalf("mode=%s", mode)
				}
				if explicitTenantID != "pick-me" {
					t.Fatalf("explicit=%s", explicitTenantID)
				}
				if cache.Param.Ext == nil {
					cache.Param.Ext = map[string]interface{}{}
				}
				cache.Param.Ext["tenant_id"] = "pick-me"
				cache.Ctx = context.WithValue(cache.Ctx, "typed-tenant", "pick-me")
				return "pick-me", nil
			},
		},
	}
	cache := &models.ApplicationCache{
		Param: &models.CommonSystemParams{},
		Ctx:   context.Background(),
	}
	tid, err := s.ApplyFunctionTenantScope(context.Background(), cache, models.FunctionTenantScopeDraftTest, "pick-me")
	if err != nil {
		t.Fatal(err)
	}
	if tid != "pick-me" {
		t.Fatalf("got %q", tid)
	}
	if cache.Param.Ext["tenant_id"] != "pick-me" {
		t.Fatalf("ext not set: %#v", cache.Param.Ext)
	}
}

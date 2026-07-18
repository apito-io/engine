package resolver

import (
	"testing"

	"github.com/apito-io/engine/models"
)

func TestRequireFunctionManage(t *testing.T) {
	cases := []struct {
		name    string
		cache   *models.ApplicationCache
		wantErr bool
	}{
		{name: "nil", cache: nil, wantErr: true},
		{
			name: "admin",
			cache: &models.ApplicationCache{
				Param: &models.CommonSystemParams{Role: &models.Role{ID: "admin", IsAdmin: true}},
			},
			wantErr: false,
		},
		{
			name: "owner",
			cache: &models.ApplicationCache{
				Param: &models.CommonSystemParams{Role: &models.Role{ID: "owner"}},
			},
			wantErr: false,
		},
		{
			name: "project_admin",
			cache: &models.ApplicationCache{
				Param: &models.CommonSystemParams{Role: &models.Role{ID: "project_admin"}},
			},
			wantErr: false,
		},
		{
			name: "content team",
			cache: &models.ApplicationCache{
				Param: &models.CommonSystemParams{Role: &models.Role{ID: "team"}},
			},
			wantErr: true,
		},
		{
			name: "app user none",
			cache: &models.ApplicationCache{
				Param: &models.CommonSystemParams{Role: &models.Role{ID: "none"}},
			},
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requireFunctionManage(tc.cache)
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

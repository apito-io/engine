package functions

import (
	"errors"
	"testing"
)

func TestTenantScopeError(t *testing.T) {
	err := NewTenantScopeError(TenantRequired, "Please select a tenant")
	if err.Error() != "TENANT_REQUIRED: Please select a tenant" {
		t.Fatalf("got %q", err.Error())
	}
	typed, ok := AsTenantScopeError(err)
	if !ok || typed.Code != TenantRequired {
		t.Fatalf("AsTenantScopeError failed: %#v ok=%v", typed, ok)
	}
	if _, ok := AsTenantScopeError(errors.New("other")); ok {
		t.Fatal("expected false for plain error")
	}
}

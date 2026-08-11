package models

import "testing"

func TestEffectiveIdleTenantRetentionDays(t *testing.T) {
	if got := EffectiveIdleTenantRetentionDays(nil); got != MinIdleTenantRetentionDays {
		t.Fatalf("nil: got %d", got)
	}
	if got := EffectiveIdleTenantRetentionDays(&ProjectSettings{}); got != MinIdleTenantRetentionDays {
		t.Fatalf("zero: got %d", got)
	}
	if got := EffectiveIdleTenantRetentionDays(&ProjectSettings{IdleTenantRetentionDays: 30}); got != MinIdleTenantRetentionDays {
		t.Fatalf("below min clamps: got %d", got)
	}
	if got := EffectiveIdleTenantRetentionDays(&ProjectSettings{IdleTenantRetentionDays: 120}); got != 120 {
		t.Fatalf("valid: got %d", got)
	}
}

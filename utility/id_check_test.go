package utility

import "testing"

func TestIsValidIdentifier_camelCase(t *testing.T) {
	v, err := IsValidIdentifier("subscriptionPlanDraft")
	if err != nil {
		t.Fatal(err)
	}
	if v.Identifier != "subscription_plan_draft" {
		t.Fatalf("identifier = %q, want subscription_plan_draft", v.Identifier)
	}
	if v.Label != "subscriptionPlanDraft" {
		t.Fatalf("label = %q", v.Label)
	}
}

func TestIsValidIdentifier_spacedAndSnake(t *testing.T) {
	cases := []struct {
		in  string
		id  string
	}{
		{"Pro Request Email", "pro_request_email"},
		{"pro_request_email", "pro_request_email"},
		{"proRequestEmail", "pro_request_email"},
		{"Weight (KG)", "weight_kg"},
	}
	for _, tc := range cases {
		v, err := IsValidIdentifier(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if v.Identifier != tc.id {
			t.Fatalf("%q: identifier = %q, want %q", tc.in, v.Identifier, tc.id)
		}
	}
}

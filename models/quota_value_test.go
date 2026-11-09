package models

import (
	"encoding/json"
	"testing"
)

func TestQuotaValueJSONRoundTrip(t *testing.T) {
	raw := []byte(`{"id":"ultra","quotas":{"max_records.food_order":1000,"max_records.payment":"M3500","max_app_users":-1}}`)
	var plan Plan
	if err := json.Unmarshal(raw, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Quotas["max_records.food_order"] != "1000" {
		t.Fatalf("number quota = %q", plan.Quotas["max_records.food_order"])
	}
	if plan.Quotas["max_records.payment"] != "M3500" {
		t.Fatalf("monthly quota = %q", plan.Quotas["max_records.payment"])
	}
	if plan.Quotas["max_app_users"] != "-1" {
		t.Fatalf("negative quota = %q", plan.Quotas["max_app_users"])
	}
	out, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	var again map[string]interface{}
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	quotas := again["quotas"].(map[string]interface{})
	if quotas["max_records.food_order"] != float64(1000) {
		t.Fatalf("marshaled number = %#v", quotas["max_records.food_order"])
	}
	if quotas["max_records.payment"] != "M3500" {
		t.Fatalf("marshaled monthly = %#v", quotas["max_records.payment"])
	}
	if quotas["max_app_users"] != float64(-1) {
		t.Fatalf("marshaled -1 = %#v", quotas["max_app_users"])
	}
}

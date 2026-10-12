package models

import (
	"encoding/json"
	"testing"
)

func TestPlanJSONPreservesHostCommercialKeys(t *testing.T) {
	raw := []byte(`{"id":"paid","name":"Paid","catalog_ref":"sku_123","prices":[{"currency":"USD","amount":25}]}`)
	var p Plan
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if p.ID != "paid" || p.Name != "Paid" {
		t.Fatalf("public fields: %+v", p)
	}
	out, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var again map[string]interface{}
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	if again["catalog_ref"] != "sku_123" {
		t.Fatalf("host key dropped: %s", out)
	}
	prices, ok := again["prices"].([]interface{})
	if !ok || len(prices) != 1 {
		t.Fatalf("prices dropped: %s", out)
	}
}

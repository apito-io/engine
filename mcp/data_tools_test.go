package mcp

import (
	"strings"
	"testing"
)

func TestListDataReturnsRecordsAndClampsLimit(t *testing.T) {
	q, ok := operationQuery("list_data")
	if !ok || !strings.Contains(q, "results {") || !strings.Contains(q, "data") {
		t.Fatalf("list_data must select results with data, got %q", q)
	}
	cases := map[any]int{nil: listDataDefaultLimit, 0: listDataDefaultLimit, 5: 5, float64(500): listDataMaxLimit}
	for in, want := range cases {
		got := aliasArgs("list_data", map[string]any{"model": "task", "limit": in})["limit"]
		if got != want {
			t.Fatalf("limit %v: got %v want %d", in, got, want)
		}
	}
}

func TestGetDataFetchesOneDocumentByID(t *testing.T) {
	q, _ := operationQuery("get_data")
	if !strings.Contains(q, "getSingleData") || !strings.Contains(q, "$_id: String!") {
		t.Fatalf("get_data must fetch one document by _id, got %q", q)
	}
	args := aliasArgs("get_data", map[string]any{"model_name": "task", "id": "doc-1"})
	if args["_id"] != "doc-1" || args["model"] != "task" {
		t.Fatalf("aliases not applied: %v", args)
	}
	vars := variablesFor(q, args)
	if vars["_id"] != "doc-1" {
		t.Fatalf("_id not passed as a variable: %v", vars)
	}
}

func TestDataToolsPublishTypedSchemas(t *testing.T) {
	for _, tool := range Catalog() {
		if tool.Name != "list_data" && tool.Name != "get_data" {
			continue
		}
		s := tool.schema()
		props := s["properties"].(map[string]any)
		if _, ok := props["model"]; !ok {
			t.Fatalf("%s schema must document model", tool.Name)
		}
		if _, ok := s["required"]; !ok {
			t.Fatalf("%s schema must list required args", tool.Name)
		}
		if strings.HasPrefix(tool.Description, "Apito MCP tool ") {
			t.Fatalf("%s needs a real description", tool.Name)
		}
	}
}

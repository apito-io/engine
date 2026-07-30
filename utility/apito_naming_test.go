package utility

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCanonicalSystemRelationFieldIdentifier(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"system_foodCategory_id", "system_food_category_id"},
		{"system_food_category_id", "system_food_category_id"},
		{"system_restaurant_id", "system_restaurant_id"},
		{"name", "name"},
		{"system_foodCategory_as_primary_id", "system_food_category_as_primary_id"},
	}
	for _, tc := range cases {
		if got := CanonicalSystemRelationFieldIdentifier(tc.in); got != tc.want {
			t.Errorf("CanonicalSystemRelationFieldIdentifier(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestCanonicalizeModelName(t *testing.T) {
	cases := []struct {
		raw     string
		want    string
		wantErr bool
	}{
		{"food_order", "food_order", false},
		{"foodOrder", "food_order", false},
		{"food Orders", "food_order", false},
		{"food-order", "food_order", false},
		{"bank_accounts", "bank_account", false},
		{"BankAccounts", "bank_account", false},
		{"tag", "tag", false},
		{"category", "category", false},
		{"users", "users", false},
		{"Users", "users", false},
		{"Indication", "indication", false},
		// Already-canonical long single words must succeed (CLI schema sync sends these).
		{"indication", "indication", false},
		{"practitioner", "practitioner", false},
		{"prescription", "prescription", false},
		{"requisition", "requisition", false},
		{"", "", true},
	}
	for _, tc := range cases {
		got, err := CanonicalizeModelName(tc.raw)
		if tc.wantErr {
			if err == nil {
				t.Errorf("CanonicalizeModelName(%q): want error", tc.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("CanonicalizeModelName(%q): %v", tc.raw, err)
			continue
		}
		if got != tc.want {
			t.Errorf("CanonicalizeModelName(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
	if err := rejectRunOnLowercaseConcat("foodorder"); !errors.Is(err, ErrRunOnModelName) {
		t.Errorf("expected ErrRunOnModelName for free-text foodorder, got %v", err)
	}
	if got, err := LegacyStoredNameToCanonical("indication"); err != nil || got != "indication" {
		t.Errorf("LegacyStoredNameToCanonical(indication) = %q, %v", got, err)
	}
}

func TestPascalFromAnyModelID(t *testing.T) {
	if got := PascalFromAnyModelID("food_order"); got != "FoodOrder" {
		t.Errorf("PascalFromAnyModelID(food_order) = %q", got)
	}
	if got := PascalFromAnyModelID("foodCategory"); got != "FoodCategory" {
		t.Errorf("PascalFromAnyModelID(foodCategory) = %q", got)
	}
}

func TestSyntheticSystemRelationFieldIdentifier(t *testing.T) {
	if got := SyntheticSystemRelationFieldIdentifier("food_category", ""); got != "system_food_category_id" {
		t.Errorf("SyntheticSystemRelationFieldIdentifier(food_category) = %q", got)
	}
	if got := SyntheticSystemRelationFieldIdentifier("foodCategory", ""); got != "system_food_category_id" {
		t.Errorf("SyntheticSystemRelationFieldIdentifier(foodCategory) = %q", got)
	}
	if got := SyntheticSystemRelationFieldIdentifier("chef", "primary_role"); got != "system_chef_as_primary_role_id" {
		t.Errorf("got %q", got)
	}
}

func TestPhysicalSQLTableName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"food_order", "food_order"},
		{"foodOrder", "food_order"},
		{"tenant", "tenant"},
		{"document_revisions", "document_revisions"},
	}
	for _, tc := range cases {
		if got := PhysicalSQLTableName(tc.in); got != tc.want {
			t.Errorf("PhysicalSQLTableName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	// Non-canonical input should match CanonicalizeModelName (same path as create-model).
	for _, raw := range []string{"foodOrder", "Food Orders", "bankAccounts"} {
		want, err := CanonicalizeModelName(raw)
		if err != nil {
			continue
		}
		if got := PhysicalSQLTableName(raw); got != want {
			t.Errorf("PhysicalSQLTableName(%q) = %q, want CanonicalizeModelName=%q", raw, got, want)
		}
	}
}

func TestNamingVectorsJSON(t *testing.T) {
	dir := filepath.Join("testdata", "naming_vectors.json")
	b, err := os.ReadFile(dir)
	if err != nil {
		t.Skip("naming_vectors.json not present:", err)
	}
	var rows []struct {
		Input                      string `json:"input"`
		Canonical                  string `json:"canonical"`
		Camel                      string `json:"camel"`
		Pascal                     string `json:"pascal"`
		ListPascal                 string `json:"listPascal"`
		CreateMutation             string `json:"createMutation"`
		ListWherePayloadUpper      string `json:"listWherePayloadUpper"`
		ListSortPayloadUpper       string `json:"listSortPayloadUpper"`
		ListKeyConditionUpper      string `json:"listKeyConditionUpper"`
		ConnectionFilterUpper      string `json:"connectionFilterUpper"`
		ListCountFilterName        string `json:"listCountFilterName"`
		ListCountWherePayloadUpper string `json:"listCountWherePayloadUpper"`
	}
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		c, err := CanonicalizeModelName(row.Input)
		if err != nil {
			t.Fatalf("CanonicalizeModelName(%q): %v", row.Input, err)
		}
		if c != row.Canonical {
			t.Errorf("canonical %q: got %q want %q", row.Input, c, row.Canonical)
		}
		if CamelFromCanonical(c) != row.Camel {
			t.Errorf("CamelFromCanonical(%q): got %q want %q", c, CamelFromCanonical(c), row.Camel)
		}
		if PascalFromAnyModelID(c) != row.Pascal {
			t.Errorf("PascalFromAnyModelID(%q): got %q want %q", c, PascalFromAnyModelID(c), row.Pascal)
		}
		if ListGraphQLTypeName(c) != row.ListPascal {
			t.Errorf("ListGraphQLTypeName(%q): got %q want %q", c, ListGraphQLTypeName(c), row.ListPascal)
		}
		wantCreate := "Create_" + row.Pascal
		if wantCreate != row.CreateMutation {
			t.Errorf("create mutation for %q: expected metadata %q", row.Input, row.CreateMutation)
		}

		// Parity with objects.BuildFilterArgument / BuildConnectionArguments string rules
		// (open-core/schemas/objects/search_filter_arg.go).
		listName := GraphQLTypeNameForFilterArg(c)
		if row.ListWherePayloadUpper != "" {
			if got := strings.ToUpper(listName + "_Input_Where_Payload"); got != row.ListWherePayloadUpper {
				t.Errorf("list where payload type for %q: got %q want %q", row.Input, got, row.ListWherePayloadUpper)
			}
			if got := strings.ToUpper(listName + "_Input_Sort_Payload"); got != row.ListSortPayloadUpper {
				t.Errorf("list sort payload type for %q: got %q want %q", row.Input, got, row.ListSortPayloadUpper)
			}
			if got := strings.ToUpper(listName + "_Key_Condition"); got != row.ListKeyConditionUpper {
				t.Errorf("list _key type for %q: got %q want %q", row.Input, got, row.ListKeyConditionUpper)
			}
		}
		if row.ConnectionFilterUpper != "" {
			if got := strings.ToUpper(c + "_Connection_Filter_Condition"); got != row.ConnectionFilterUpper {
				t.Errorf("connection filter type for %q: got %q want %q", row.Input, got, row.ConnectionFilterUpper)
			}
		}
		if row.ListCountFilterName != "" {
			gotName := GraphQLComposedTypeName(c, "List_Count")
			if gotName != row.ListCountFilterName {
				t.Errorf("List_Count filter name for %q: got %q want %q", row.Input, gotName, row.ListCountFilterName)
			}
			if row.ListCountWherePayloadUpper != "" {
				if got := strings.ToUpper(gotName + "_Input_Where_Payload"); got != row.ListCountWherePayloadUpper {
					t.Errorf("count where payload type for %q: got %q want %q", row.Input, got, row.ListCountWherePayloadUpper)
				}
			}
		}
	}
}

func TestRelationFilterGraphQLKey(t *testing.T) {
	if got := RelationFilterGraphQLKey("users", "owner"); got != "owner" {
		t.Fatalf("known_as: got %q want owner", got)
	}
	if got := RelationFilterGraphQLKey("users", ""); got != "users" {
		t.Fatalf("users model: got %q want users", got)
	}
	if got := RelationFilterGraphQLKey("food_category", ""); got != "food_category" {
		t.Fatalf("food_category: got %q want food_category", got)
	}
	if got := RelationFilterGraphQLKey("ledgerAccount", ""); got != "ledger_account" {
		t.Fatalf("legacy camel ledgerAccount: got %q want ledger_account", got)
	}
	if got := RelationNestedListGraphQLKey("food_category", ""); got != "food_category_list" {
		t.Fatalf("nested list: got %q want food_category_list", got)
	}
	if got := RelationNestedListGraphQLKey("users", "chef"); got != "chef_list" {
		t.Fatalf("known_as nested list: got %q want chef_list", got)
	}
	if got := RelationNestedListGraphQLKey("food", ""); got != "food_list" {
		t.Fatalf("single-word nested list: got %q want food_list", got)
	}
}

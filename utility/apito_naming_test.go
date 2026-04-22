package utility

import (
	"encoding/json"
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
		raw    string
		want   string
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
	if _, err := CanonicalizeModelName("foodorder"); err == nil {
		t.Error("expected ErrRunOnName for foodorder")
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

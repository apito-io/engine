package utility

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

func TestExtractModelNames_foodCategoryList_resolvesCanonicalSnakeModel(t *testing.T) {
	schema := &models.ProjectSchema{
		Models: []*models.ModelType{
			{Name: "food_category", Fields: []*models.FieldInfo{{Identifier: "title"}}},
		},
	}
	q := `query MyQuery { foodCategoryList { id } }`
	doc, err := parser.ParseQuery(&ast.Source{Input: q})
	if err != nil {
		t.Fatal(err)
	}
	reqs, _, err := ExtractModelNames(schema, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("len(reqs)=%d want 1", len(reqs))
	}
	if len(reqs[0].RootFields) != 1 || reqs[0].RootFields[0] != "foodCategoryList" {
		t.Fatalf("RootFields=%#v want [foodCategoryList]", reqs[0].RootFields)
	}
	if len(reqs[0].FilteredModels) != 1 {
		t.Fatalf("FilteredModels=%#v", reqs[0].FilteredModels)
	}
	if got := reqs[0].FilteredModels[0].Name; got != "food_category" {
		t.Fatalf("want food_category, got %q", got)
	}
}

func TestExtractModelNames_authOnlyRootFields(t *testing.T) {
	schema := &models.ProjectSchema{
		Models: []*models.ModelType{
			{Name: "student", Fields: []*models.FieldInfo{{Identifier: "name"}}},
		},
	}
	q := `query { myEffectivePermissions { role_id plan_slug } }`
	doc, err := parser.ParseQuery(&ast.Source{Input: q})
	if err != nil {
		t.Fatal(err)
	}
	reqs, _, err := ExtractModelNames(schema, doc)
	if err != nil {
		t.Fatal(err)
	}
	if len(reqs) != 1 {
		t.Fatalf("len(reqs)=%d want 1", len(reqs))
	}
	if len(reqs[0].FilteredModels) != 0 {
		t.Fatalf("FilteredModels=%#v want empty for auth-only root", reqs[0].FilteredModels)
	}
	if len(reqs[0].RootFields) != 1 || reqs[0].RootFields[0] != "myEffectivePermissions" {
		t.Fatalf("RootFields=%#v", reqs[0].RootFields)
	}
}

func TestResolveStoredModelID(t *testing.T) {
	known := map[string]bool{"food_category": true, "legacyCamel": true}
	if got := ResolveStoredModelID(known, "foodCategory"); got != "food_category" {
		t.Fatalf("foodCategory -> %q want food_category", got)
	}
	if got := ResolveStoredModelID(known, "food_category"); got != "food_category" {
		t.Fatalf("food_category -> %q want food_category", got)
	}
	if got := ResolveStoredModelID(known, "food_order"); got != "" {
		t.Fatalf("unknown -> %q want empty", got)
	}
}

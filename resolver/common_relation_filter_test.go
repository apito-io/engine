package resolver

import (
	"testing"

	"github.com/apito-io/engine/models"
)

func TestRelationFilterHasFieldPredicates(t *testing.T) {
	if relationFilterHasFieldPredicates(map[string]interface{}{"_id": map[string]interface{}{"eq": "m1"}}) {
		t.Fatal("id-only relation filter must stay native")
	}
	if !relationFilterHasFieldPredicates(map[string]interface{}{
		"name": map[string]interface{}{"contains": "para"},
	}) {
		t.Fatal("field filters must trigger two-phase rewrite")
	}
}

func TestConnectionForRelationKey(t *testing.T) {
	stock := &models.ModelType{
		Name: "stock",
		Connections: []*models.ConnectionType{
			{Model: "medicine", Relation: "has_one"},
		},
	}
	if connectionForRelationKey(stock, "medicine") == nil {
		t.Fatal("expected medicine connection")
	}
	if connectionForRelationKey(stock, "patient") != nil {
		t.Fatal("unexpected connection")
	}
}

func TestFindSchemaModelByName(t *testing.T) {
	modelsList := []*models.ModelType{{Name: "medicine"}, {Name: "stock"}}
	if findSchemaModelByName(modelsList, "medicine") == nil {
		t.Fatal("expected medicine")
	}
	if findSchemaModelByName(modelsList, "Medicine") == nil {
		t.Fatal("case-insensitive match")
	}
}

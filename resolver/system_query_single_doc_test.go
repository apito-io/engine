package resolver

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
)

func TestSingleProjectDocumentResult_MissingRowIsNull(t *testing.T) {
	got := singleProjectDocumentResult(nil, &models.ModelType{Name: "food_order"}, false)
	if got != nil {
		t.Fatalf("got %#v, want nil", got)
	}
}

func TestSingleProjectDocumentResult_MissingSinglePageIsEmptyDoc(t *testing.T) {
	model := &models.ModelType{Name: "settings", SinglePageUUID: "page-1"}
	got := singleProjectDocumentResult(nil, model, true)
	doc, ok := got.(*types.DefaultDocumentStructure)
	if !ok || doc == nil {
		t.Fatalf("got %#v, want empty document", got)
	}
	if doc.ID != "page-1" || doc.Type != "settings" {
		t.Fatalf("doc = %+v", doc)
	}
}

func TestSingleProjectDocumentResult_FoundDocPassesThrough(t *testing.T) {
	in := &types.DefaultDocumentStructure{ID: "01M3A2225C3RJNCKM24AAF4QQ9", Type: "food_order"}
	got := singleProjectDocumentResult(in, &models.ModelType{Name: "food_order"}, false)
	if got != in {
		t.Fatal("found document was replaced")
	}
}

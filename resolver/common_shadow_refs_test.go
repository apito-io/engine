package resolver

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/tailor-platform/graphql"
)

func TestCommonModelShadowRefSources(t *testing.T) {
	common := &models.ModelType{Name: "catalog_item", Ext: map[string]interface{}{models.ExtKeyIsCommonModel: true}}
	tenant := &models.ModelType{
		Name: "tenant_record",
		Connections: []*models.ConnectionType{
			{Model: "catalog_item", Relation: "has_one", Type: "forward"},
		},
	}
	got := commonModelShadowRefSources([]*models.ModelType{common, tenant}, common)
	if len(got) != 1 || got[0].Table != "tenant_record" || got[0].Column != "catalog_item_id" {
		t.Fatalf("got %#v", got)
	}
}

func TestRejectUnsupportedCommonRelationSort(t *testing.T) {
	param := &models.CommonSystemParams{
		Model: &models.ModelType{
			Name: "tenant_record",
			Fields: []*models.FieldInfo{
				{Identifier: "title", FieldType: "text"},
			},
			Connections: []*models.ConnectionType{
				{Model: "catalog_item", Relation: "has_one"},
			},
		},
		ProjectSchemaModels: []*models.ModelType{
			{Name: "catalog_item", Ext: map[string]interface{}{models.ExtKeyIsCommonModel: true}},
			{Name: "tenant_record"},
		},
		ResolveParams: &graphql.ResolveParams{
			Args: map[string]interface{}{"sort": map[string]interface{}{"catalog_item": "ASC"}},
		},
	}
	if err := rejectUnsupportedCommonRelationSort(param); err == nil {
		t.Fatal("expected sort-by-common-relation error")
	}
	param.ResolveParams.Args["sort"] = map[string]interface{}{"title": "ASC"}
	if err := rejectUnsupportedCommonRelationSort(param); err != nil {
		t.Fatal(err)
	}
}

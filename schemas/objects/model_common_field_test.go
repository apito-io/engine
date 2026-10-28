package objects

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/tailor-platform/graphql"
)

func TestResolveModelIsCommon(t *testing.T) {
	common := &models.ModelType{Name: "catalog", Ext: map[string]interface{}{models.ExtKeyIsCommonModel: true}}
	got, err := resolveModelIsCommon(graphql.ResolveParams{Source: common})
	if err != nil || got != true {
		t.Fatalf("common model = %v, %v", got, err)
	}

	plain := &models.ModelType{Name: "note"}
	got, err = resolveModelIsCommon(graphql.ResolveParams{Source: plain})
	if err != nil || got != false {
		t.Fatalf("plain model = %v, %v", got, err)
	}

	got, err = resolveModelIsCommon(graphql.ResolveParams{Source: models.ModelType{Ext: map[string]interface{}{models.ExtKeyIsCommonModel: true}}})
	if err != nil || got != true {
		t.Fatalf("value model = %v, %v", got, err)
	}
}

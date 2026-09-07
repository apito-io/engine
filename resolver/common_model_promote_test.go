package resolver

import (
	"strings"
	"testing"

	"github.com/apito-io/types"
)

func TestValidateCommonModelToggle(t *testing.T) {
	if err := validateCommonModelToggle(false, "demote", true, "medicine"); err == nil {
		t.Fatal("demotion must be rejected")
	}
	if err := validateCommonModelToggle(true, "demote", true, "medicine"); err == nil {
		t.Fatal("demote strategy must be rejected")
	}
	if err := validateCommonModelToggle(true, "", true, "medicine"); err == nil {
		t.Fatal("rows without promote must be rejected")
	}
	if err := validateCommonModelToggle(true, "promote", true, "medicine"); err != nil {
		t.Fatalf("promote with rows: %v", err)
	}
	if err := validateCommonModelToggle(true, "", false, "medicine"); err != nil {
		t.Fatalf("empty model can become common: %v", err)
	}
}

func TestRunCommonModelPromotionPaginatesAndResumes(t *testing.T) {
	base := map[string]*types.DefaultDocumentStructure{}
	tenant := []*types.DefaultDocumentStructure{
		{ID: "a", Data: map[string]interface{}{"name": "A"}},
		{ID: "b", Data: map[string]interface{}{"name": "B"}},
		{ID: "c", Data: map[string]interface{}{"name": "C"}},
		{ID: "d", Data: map[string]interface{}{"name": "D"}},
		{ID: "e", Data: map[string]interface{}{"name": "E"}},
	}
	io := commonPromoteIO{
		listTenant: func(page, limit int) ([]*types.DefaultDocumentStructure, error) {
			start := (page - 1) * limit
			if start >= len(tenant) {
				return nil, nil
			}
			end := start + limit
			if end > len(tenant) {
				end = len(tenant)
			}
			return tenant[start:end], nil
		},
		getBase: func(id string) (*types.DefaultDocumentStructure, error) {
			return base[id], nil
		},
		addBase: func(doc *types.DefaultDocumentStructure) error {
			cp := *doc
			base[doc.ID] = &cp
			return nil
		},
		baseHas: func(id string) (bool, error) {
			_, ok := base[id]
			return ok, nil
		},
		reduce: func() error { return nil },
	}
	n, didReduce, err := runCommonModelPromotion(2, io)
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 || !didReduce || len(base) != 5 {
		t.Fatalf("first pass promoted=%d reduced=%v base=%d", n, didReduce, len(base))
	}
	n, didReduce, err = runCommonModelPromotion(2, io)
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 || !didReduce {
		t.Fatalf("resume should skip existing ids, got promoted=%d reduced=%v", n, didReduce)
	}
}

func TestRunCommonModelPromotionDuplicateConflict(t *testing.T) {
	base := map[string]*types.DefaultDocumentStructure{
		"dup": {ID: "dup", Data: map[string]interface{}{"name": "A"}},
	}
	io := commonPromoteIO{
		listTenant: func(page, limit int) ([]*types.DefaultDocumentStructure, error) {
			if page > 1 {
				return nil, nil
			}
			return []*types.DefaultDocumentStructure{
				{ID: "dup", Data: map[string]interface{}{"name": "B"}},
			}, nil
		},
		getBase: func(id string) (*types.DefaultDocumentStructure, error) {
			return base[id], nil
		},
		addBase: func(doc *types.DefaultDocumentStructure) error {
			t.Fatal("must not insert conflicting id")
			return nil
		},
		baseHas: func(id string) (bool, error) {
			_, ok := base[id]
			return ok, nil
		},
		reduce: func() error {
			t.Fatal("must not reduce after conflict")
			return nil
		},
	}
	_, _, err := runCommonModelPromotion(10, io)
	if err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("want conflict, got %v", err)
	}
}

func TestRunCommonModelPromotionSkipsReduceWhenVerifyFails(t *testing.T) {
	io := commonPromoteIO{
		listTenant: func(page, limit int) ([]*types.DefaultDocumentStructure, error) {
			if page > 1 {
				return nil, nil
			}
			return []*types.DefaultDocumentStructure{{ID: "x", Data: map[string]interface{}{"n": 1}}}, nil
		},
		getBase: func(id string) (*types.DefaultDocumentStructure, error) { return nil, nil },
		addBase: func(doc *types.DefaultDocumentStructure) error { return nil },
		baseHas: func(id string) (bool, error) { return false, nil },
		reduce: func() error {
			t.Fatal("reduce must not run")
			return nil
		},
	}
	_, reduced, err := runCommonModelPromotion(10, io)
	if err == nil || reduced {
		t.Fatalf("want verify error without reduce, got reduced=%v err=%v", reduced, err)
	}
}

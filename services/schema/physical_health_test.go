package schema

import (
	"reflect"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

func TestExpectedPhysicalColumns_TopLevelAndLocals(t *testing.T) {
	model := &models.ModelType{
		Name: "app_release_policy",
		Fields: []*models.FieldInfo{
			{Identifier: "platform", FieldType: _const.TextField, InputType: "string"},
			{Identifier: "force_update", FieldType: _const.BooleanField, InputType: "bool"},
			{Identifier: "name", FieldType: _const.TextField, InputType: "string", Validation: &models.Validation{Locals: []string{"en", "bn"}}},
			{Identifier: "nested", FieldType: _const.TextField, InputType: "string", ParentField: "payload"},
		},
	}
	got := ExpectedPhysicalColumns(model)
	want := []string{"force_update", "id", "name", "name_bn", "platform"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestBuildModelPhysicalHealth_StubTable(t *testing.T) {
	model := &models.ModelType{
		Name: "app_release_policy",
		Ext:  map[string]interface{}{"is_common_model": true},
		Fields: []*models.FieldInfo{
			{Identifier: "platform", FieldType: _const.TextField, InputType: "string"},
			{Identifier: "force_update", FieldType: _const.BooleanField, InputType: "bool"},
		},
	}
	h := BuildModelPhysicalHealth(model, true, []string{"id"})
	if !h.TableExists {
		t.Fatal("expected table_exists")
	}
	if !h.IsCommonModel {
		t.Fatal("expected is_common_model")
	}
	if len(h.MissingColumns) < 2 {
		t.Fatalf("expected missing columns, got %v", h.MissingColumns)
	}
	foundStub := false
	for _, w := range h.Warnings {
		if contains(w, "only id column") {
			foundStub = true
		}
	}
	if !foundStub {
		t.Fatalf("expected stub warning, got %v", h.Warnings)
	}
}

func TestBuildModelPhysicalHealth_MissingTable(t *testing.T) {
	model := &models.ModelType{Name: "x", Fields: []*models.FieldInfo{
		{Identifier: "a", FieldType: _const.TextField, InputType: "string"},
	}}
	h := BuildModelPhysicalHealth(model, false, nil)
	if h.TableExists {
		t.Fatal("expected missing table")
	}
	if len(h.Warnings) == 0 {
		t.Fatal("expected warning")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 ||
		(func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		})())
}

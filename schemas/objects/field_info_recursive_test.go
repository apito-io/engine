package objects

import (
	"testing"

	"github.com/tailor-platform/graphql"
)

func TestGetFieldInfoObject_SubFieldInfoIsRecursive(t *testing.T) {
	s := &SchemaObjects{}
	validation := s.GetValidationTypeObject()
	fieldInfo := s.GetFieldInfoObject(validation)

	rootFields := fieldInfo.Fields()
	subList, ok := rootFields["sub_field_info"]
	if !ok || subList == nil {
		t.Fatal("FieldInfo missing sub_field_info")
	}
	list, ok := subList.Type.(*graphql.List)
	if !ok {
		t.Fatalf("sub_field_info type = %T, want *graphql.List", subList.Type)
	}
	subObj, ok := list.OfType.(*graphql.Object)
	if !ok {
		t.Fatalf("sub_field_info element = %T, want *graphql.Object", list.OfType)
	}
	if subObj.Name() != "SubFieldInfo" {
		t.Fatalf("nested type name = %q, want SubFieldInfo", subObj.Name())
	}
	nested := subObj.Fields()
	nestedSub, ok := nested["sub_field_info"]
	if !ok || nestedSub == nil {
		t.Fatal("SubFieldInfo must expose recursive sub_field_info (was NestedSubFieldInfo leaf)")
	}
	nestedList, ok := nestedSub.Type.(*graphql.List)
	if !ok {
		t.Fatalf("SubFieldInfo.sub_field_info type = %T, want *graphql.List", nestedSub.Type)
	}
	if nestedList.OfType != subObj {
		t.Fatalf("SubFieldInfo.sub_field_info must recurse to same SubFieldInfo type")
	}
}

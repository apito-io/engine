package objects

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

func TestBuildWhereConditionArgument_loanInstallmentCollision(t *testing.T) {
	field := &models.FieldInfo{
		Identifier: "installment_amount",
		FieldType:  "number",
		InputType:  "double",
	}
	t1 := BuildWhereConditionArgument("loan", "installment_amount", field)
	t2 := BuildWhereConditionArgument("loan_installment", "amount", field)
	if t1 == nil || t2 == nil {
		t.Fatal("expected non-nil types")
	}
	if t1.Name() == t2.Name() {
		t.Fatalf("duplicate graphql type names: %s", t1.Name())
	}
	wantLoan := utility.WhereFilterConditionGraphQLTypeName("loan", "installment_amount")
	wantLI := utility.WhereFilterConditionGraphQLTypeName("loan_installment", "amount")
	if t1.Name() != wantLoan {
		t.Fatalf("loan type: got %q want %q", t1.Name(), wantLoan)
	}
	if t2.Name() != wantLI {
		t.Fatalf("loan_installment type: got %q want %q", t2.Name(), wantLI)
	}
}

func TestBuildWhereConditionArgument_EmptyNestedRepeatedDoesNotPanic(t *testing.T) {
	// Prod Protiva shape: exam.routine.details with no children crashed public schema.
	details := &models.FieldInfo{
		Identifier:   "details",
		FieldType:    "repeated",
		InputType:    "repeated",
		SubFieldInfo: nil,
	}
	got := BuildWhereConditionArgument("exam", "routine__details__repeated", details)
	if got == nil {
		t.Fatal("expected placeholder input object")
	}
	fields := got.Fields()
	if _, ok := fields["_empty"]; !ok {
		t.Fatalf("expected _empty placeholder, got %#v", fields)
	}
}

func TestBuildWhereConditionArgument_NestedRepeatedWithChildren(t *testing.T) {
	details := &models.FieldInfo{
		Identifier: "details",
		FieldType:  "repeated",
		InputType:  "repeated",
		SubFieldInfo: []*models.FieldInfo{
			{Identifier: "date_and_time", FieldType: "date", InputType: "string"},
			{Identifier: "subject_code", FieldType: "text", InputType: "string"},
		},
	}
	got := BuildWhereConditionArgument("exam", "routine__details__repeated", details)
	fields := got.Fields()
	if _, ok := fields["date_and_time"]; !ok {
		t.Fatal("missing date_and_time filter")
	}
	if _, ok := fields["subject_code"]; !ok {
		t.Fatal("missing subject_code filter")
	}
	if _, ok := fields["_empty"]; ok {
		t.Fatal("populated nested group must not use _empty placeholder")
	}
}

func TestBuildWhereConditionArgument_EmptyNestedObjectDoesNotPanic(t *testing.T) {
	meta := &models.FieldInfo{
		Identifier:   "meta",
		FieldType:    "object",
		InputType:    "object",
		SubFieldInfo: nil,
	}
	got := BuildWhereConditionArgument("exam", "routine__meta__object", meta)
	if got == nil {
		t.Fatal("expected placeholder input object")
	}
	fields := got.Fields()
	if _, ok := fields["_empty"]; !ok {
		t.Fatalf("expected _empty placeholder for empty object, got %#v", fields)
	}
}

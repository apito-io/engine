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

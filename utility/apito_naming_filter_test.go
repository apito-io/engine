package utility

import "testing"

func TestWhereFilterConditionGraphQLTypeName_noCollision(t *testing.T) {
	a := WhereFilterConditionGraphQLTypeName("loan_installment", "amount")
	b := WhereFilterConditionGraphQLTypeName("loan", "installment_amount")
	if a == b {
		t.Fatalf("types must differ: both %q", a)
	}
	if a != "LOAN_INSTALLMENT__FIELD__AMOUNT__COMMON_FILTER_CONDITION" {
		t.Fatalf("got %q", a)
	}
	if b != "LOAN__FIELD__INSTALLMENT_AMOUNT__COMMON_FILTER_CONDITION" {
		t.Fatalf("got %q", b)
	}
}

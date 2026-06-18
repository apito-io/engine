package resolver

import (
	"testing"

	"github.com/tailor-platform/graphql"
)

func TestParseGraphQLStringListArg(t *testing.T) {
	t.Parallel()

	args := map[string]interface{}{
		"models": []interface{}{" tenant ", "food_order", "", nil, 42},
	}
	got := parseGraphQLStringListArg(args, "models")
	want := []string{"tenant", "food_order"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	if empty := parseGraphQLStringListArg(map[string]interface{}{}, "models"); empty != nil {
		t.Fatalf("expected nil for missing arg, got %v", empty)
	}
}

func TestCloneResolveParamsForModelCount(t *testing.T) {
	t.Parallel()

	parent := graphql.ResolveParams{
		Args: map[string]interface{}{
			"search": "hello",
		},
	}
	child := cloneResolveParamsForModelCount(parent, "tenant")
	if child.Args["model"] != "tenant" {
		t.Fatalf("model = %v", child.Args["model"])
	}
	if child.Args["status"] != "all" {
		t.Fatalf("status = %v", child.Args["status"])
	}
	if child.Args["search"] != "hello" {
		t.Fatalf("search = %v", child.Args["search"])
	}
}

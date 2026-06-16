package models

import "testing"

func TestNormalizeProjectSchemaConnectionTypes_reverseToBackward(t *testing.T) {
	schema := &ProjectSchema{
		Models: []*ModelType{
			{
				Name: "vendor_profile",
				Connections: []*ConnectionType{
					{Model: "tenant", Type: "reverse", Relation: "has_one", KnownAs: "ownerProfile"},
				},
			},
		},
	}
	NormalizeProjectSchemaConnectionTypes(schema)
	if got := schema.Models[0].Connections[0].Type; got != "backward" {
		t.Fatalf("Type = %q, want backward", got)
	}
}

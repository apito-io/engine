package objects

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"github.com/tailor-platform/graphql"
)

func TestBuildWhereRelationConditionArgument_knownAsKey(t *testing.T) {
	connections := []*models.ConnectionType{
		{Model: "users", Relation: "has_one", KnownAs: "owner"},
	}
	whereArgs := map[string]graphql.InputObjectConfigFieldMap{
		"users": {
			"email": &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	}

	obj := BuildWhereRelationConditionArgument("user_profile", connections, whereArgs)
	require.NotNil(t, obj)
	fields := obj.Fields()
	require.Contains(t, fields, "owner")
	require.NotContains(t, fields, "users")
}

func TestBuildWhereRelationConditionArgument_twoKnownAsSameModel(t *testing.T) {
	connections := []*models.ConnectionType{
		{Model: "employee", Relation: "has_one", KnownAs: "chef"},
		{Model: "employee", Relation: "has_one", KnownAs: "waiter"},
	}
	whereArgs := map[string]graphql.InputObjectConfigFieldMap{
		"employee": {
			"name": &graphql.InputObjectFieldConfig{Type: graphql.String},
		},
	}

	obj := BuildWhereRelationConditionArgument("food_order", connections, whereArgs)
	require.NotNil(t, obj)
	fields := obj.Fields()
	require.Contains(t, fields, "chef")
	require.Contains(t, fields, "waiter")
	require.NotContains(t, fields, "employee")
}

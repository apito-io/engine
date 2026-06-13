package controller

import (
	"context"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/stretchr/testify/require"
	"github.com/tailor-platform/graphql"
)

func TestResolveConnectionPermission_knownAsKey(t *testing.T) {
	conn := &models.ConnectionType{Model: "employee", KnownAs: "chef", Relation: "has_one"}
	perms := map[string]*models.APIPermission{
		"chef": {Read: "all", Create: "none", Update: "none", Delete: "none"},
	}
	ap, ok := resolveConnectionPermission(perms, conn, nil)
	require.True(t, ok)
	require.Equal(t, "all", ap.Read)
}

func TestPublicSchemaBuilder_knownAsRelation_withParentRead(t *testing.T) {
	p := baseProject("proj-known-as")
	p.Schema = &models.ProjectSchema{
		Models: []*models.ModelType{
			{
				Name: "employee",
				Fields: []*models.FieldInfo{
					{Identifier: "name", InputType: _const.StringInput, FieldType: "string"},
				},
			},
			{
				Name: "food_order",
				Fields: []*models.FieldInfo{
					{Identifier: "note", InputType: _const.StringInput, FieldType: "string"},
				},
				Connections: []*models.ConnectionType{
					{Model: "employee", Relation: "has_one", KnownAs: "chef"},
					{Model: "employee", Relation: "has_one", KnownAs: "waiter"},
				},
			},
		},
	}
	cache := &models.ApplicationCache{
		Project: p,
		Param: &models.CommonSystemParams{
			ProjectID: p.ID,
			Role: &models.Role{
				ID:      "receptionist",
				IsAdmin: false,
				APIPermissions: map[string]*models.APIPermission{
					"food_order": {Read: "all", Create: "all", Update: "all", Delete: "none"},
				},
			},
		},
	}
	g := &GraphCtrl{cfg: &models.Config{}, gqlServer: &resolver.GraphQLServer{}}
	out, err := g.publicSchemaBuilder(context.Background(), cache)
	require.NoError(t, err)

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{
			Name:   "QueryType",
			Fields: out.RawSchemas.Queries,
		}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name:   "MutationType",
			Fields: out.RawSchemas.Mutations,
		}),
		Types: []graphql.Type{graphql.String, graphql.Int, graphql.Float, graphql.Boolean, graphql.ID},
	})
	require.NoError(t, err)

	result := graphql.Do(graphql.Params{
		Schema:        schema,
		RequestString: `{ __type(name: "FoodOrder") { fields { name } } }`,
	})
	require.Empty(t, result.Errors)
	data, ok := result.Data.(map[string]interface{})
	require.True(t, ok)
	typeNode, ok := data["__type"].(map[string]interface{})
	require.True(t, ok)
	fields, ok := typeNode["fields"].([]interface{})
	require.True(t, ok)
	names := make(map[string]bool)
	for _, f := range fields {
		fm, ok := f.(map[string]interface{})
		require.True(t, ok)
		names[fm["name"].(string)] = true
	}
	require.True(t, names["chef"], "FoodOrder should expose chef known_as relation")
	require.True(t, names["waiter"], "FoodOrder should expose waiter known_as relation")
}

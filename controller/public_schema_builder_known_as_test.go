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

// ensureUserItemGraphQLObject avoids RegisterUserSchema blocking on a nil SystemQueriesChan
// when tests construct an empty GraphQLServer.
func ensureUserItemGraphQLObject() {
	if resolver.UserItemGraphQLObject != nil {
		return
	}
	resolver.UserItemGraphQLObject = graphql.NewObject(graphql.ObjectConfig{
		Name: "UserItem",
		Fields: graphql.Fields{
			"id": &graphql.Field{Type: graphql.String},
		},
	})
}

func TestResolveConnectionPermission_knownAsKey(t *testing.T) {
	conn := &models.ConnectionType{Model: "employee", KnownAs: "chef", Relation: "has_one"}
	perms := map[string]*models.APIPermission{
		"chef": {Read: "all", Create: "none", Update: "none", Delete: "none"},
	}
	ap, ok := resolveConnectionPermission(perms, conn, nil)
	require.True(t, ok)
	require.Equal(t, "all", ap.Read)
}

func TestConnectionFieldAllowed_knownAsRequiresTargetRead(t *testing.T) {
	conn := &models.ConnectionType{Model: "employee", KnownAs: "chef", Relation: "has_one"}
	role := &models.Role{ID: "receptionist"}

	// Parent read alone must not expose known_as when target Read is missing/none.
	permsParentOnly := map[string]*models.APIPermission{
		"food_order": {Read: "all"},
	}
	require.False(t, connectionFieldAllowed(permsParentOnly, "food_order", conn, role))

	permsTargetNone := map[string]*models.APIPermission{
		"food_order": {Read: "all"},
		"chef":       {Read: "none"},
	}
	require.False(t, connectionFieldAllowed(permsTargetNone, "food_order", conn, role))

	permsOK := map[string]*models.APIPermission{
		"food_order": {Read: "all"},
		"chef":       {Read: "all"},
	}
	require.True(t, connectionFieldAllowed(permsOK, "food_order", conn, role))

	// Admin bypass even when target is none.
	admin := &models.Role{ID: "admin"}
	require.True(t, connectionFieldAllowed(permsTargetNone, "food_order", conn, admin))
}

func TestPublicSchemaBuilder_knownAsRelation_withParentRead(t *testing.T) {
	ensureUserItemGraphQLObject()
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
					// Target read required: known_as parent traverse must not expose employee when Read is none/missing.
					"chef":   {Read: "all", Create: "none", Update: "none", Delete: "none"},
					"waiter": {Read: "all", Create: "none", Update: "none", Delete: "none"},
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

func TestPublicSchemaBuilder_knownAsRelation_skipsWhenTargetReadNone(t *testing.T) {
	ensureUserItemGraphQLObject()
	p := baseProject("proj-known-as-deny")
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
					"chef":       {Read: "none", Create: "none", Update: "none", Delete: "none"},
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
	data := result.Data.(map[string]interface{})
	typeNode := data["__type"].(map[string]interface{})
	fields := typeNode["fields"].([]interface{})
	for _, f := range fields {
		fm := f.(map[string]interface{})
		require.NotEqual(t, "chef", fm["name"], "chef must not be exposed when target Read is none")
	}
}

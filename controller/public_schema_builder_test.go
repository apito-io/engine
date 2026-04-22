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

func TestSchemaRoleForPublicSchemaBuild_roleAgnosticElevates(t *testing.T) {
	r := &models.Role{IsAdmin: false, ID: "u1"}
	sup := schemaRoleForPublicSchemaBuild(r, true)
	require.True(t, sup.IsAdmin)
	same := schemaRoleForPublicSchemaBuild(r, false)
	require.False(t, same.IsAdmin)
}

func TestFingerprintPreConnection_stable(t *testing.T) {
	p := &models.Project{
		ID: "p1",
		Schema: &models.ProjectSchema{
			Models: []*models.ModelType{
				{
					Name: "post",
					Fields: []*models.FieldInfo{
						{Identifier: "title", InputType: _const.StringInput, FieldType: "string"},
					},
				},
			},
		},
	}
	role := &models.Role{IsAdmin: true}
	a := fingerprintPreConnection(p, role, nil)
	b := fingerprintPreConnection(p, role, nil)
	require.Equal(t, a, b)
}

func TestPublicSchemaBuilder_oneModel_admin(t *testing.T) {
	cfg := &models.Config{}
	g := &GraphCtrl{
		cfg: cfg,
		gqlServer: &resolver.GraphQLServer{},
	}
	ctx := context.Background()
	cache := &models.ApplicationCache{
		Project: &models.Project{
			ID: "proj-1",
			Settings: &models.ProjectSettings{
				Locals: []string{"en"},
			},
			Schema: &models.ProjectSchema{
				Models: []*models.ModelType{
					{
						Name: "article",
						Fields: []*models.FieldInfo{
							{Identifier: "title", InputType: _const.StringInput, FieldType: "string"},
						},
					},
				},
			},
		},
		Param: &models.CommonSystemParams{
			ProjectID: "proj-1",
			Role:      &models.Role{IsAdmin: true},
		},
		IncomingRequest: nil,
	}
	out, err := g.publicSchemaBuilder(ctx, cache)
	require.NoError(t, err)
	require.NotNil(t, out.RawSchemas)
	require.NotEmpty(t, out.RawSchemas.Queries)
	require.NotNil(t, out.Dataloaders["system_user_loader"])

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
	require.NotNil(t, schema)
}

func TestPublicSchemaBuilder_canonicalSnake_singleQueryRootIsCamel(t *testing.T) {
	cfg := &models.Config{}
	g := &GraphCtrl{cfg: cfg, gqlServer: &resolver.GraphQLServer{}}
	ctx := context.Background()
	cache := &models.ApplicationCache{
		Project: &models.Project{
			ID: "proj-1",
			Settings: &models.ProjectSettings{Locals: []string{"en"}},
			Schema: &models.ProjectSchema{
				Models: []*models.ModelType{
					{
						Name: "food_order",
						Fields: []*models.FieldInfo{
							{Identifier: "note", InputType: _const.StringInput, FieldType: "string"},
						},
					},
				},
			},
		},
		Param: &models.CommonSystemParams{
			ProjectID: "proj-1",
			Role:      &models.Role{IsAdmin: true},
		},
		IncomingRequest: nil,
	}
	out, err := g.publicSchemaBuilder(ctx, cache)
	require.NoError(t, err)
	require.NotNil(t, out.RawSchemas.Queries)
	require.Contains(t, out.RawSchemas.Queries, "foodOrder")
	require.Contains(t, out.RawSchemas.Queries, "foodOrderList")
	require.NotContains(t, out.RawSchemas.Queries, "food_order")
}

func TestPublicSchemaBuilder_maxModelsExceeded(t *testing.T) {
	cfg := &models.Config{MaxModelsPerProject: 1}
	g := &GraphCtrl{cfg: cfg, gqlServer: &resolver.GraphQLServer{}}
	var modelsList []*models.ModelType
	for i := 0; i < 3; i++ {
		modelsList = append(modelsList, &models.ModelType{
			Name: "m",
			Fields: []*models.FieldInfo{
				{Identifier: "x", InputType: _const.StringInput, FieldType: "string"},
			},
		})
	}
	modelsList[0].Name = "a"
	modelsList[1].Name = "b"
	modelsList[2].Name = "c"
	cache := &models.ApplicationCache{
		Project: &models.Project{
			ID: "p",
			Settings: &models.ProjectSettings{Locals: []string{"en"}},
			Schema: &models.ProjectSchema{Models: modelsList},
		},
		Param: &models.CommonSystemParams{
			ProjectID: "p",
			Role:      &models.Role{IsAdmin: true},
		},
	}
	_, err := g.publicSchemaBuilder(context.Background(), cache)
	require.Error(t, err)
}

func BenchmarkPublicSchemaBuilder_oneModel(b *testing.B) {
	cfg := &models.Config{}
	g := &GraphCtrl{cfg: cfg, gqlServer: &resolver.GraphQLServer{}}
	cache := &models.ApplicationCache{
		Project: &models.Project{
			ID: "proj-1",
			Settings: &models.ProjectSettings{
				Locals: []string{"en"},
			},
			Schema: &models.ProjectSchema{
				Models: []*models.ModelType{
					{
						Name: "article",
						Fields: []*models.FieldInfo{
							{Identifier: "title", InputType: _const.StringInput, FieldType: "string"},
						},
					},
				},
			},
		},
		Param: &models.CommonSystemParams{
			ProjectID: "proj-1",
			Role:      &models.Role{IsAdmin: true},
		},
	}
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = g.publicSchemaBuilder(ctx, cache)
	}
}

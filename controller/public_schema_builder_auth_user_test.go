package controller

import (
	"context"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/stretchr/testify/require"
)

func TestCollectFilteredModelsForPublicSchema_skipsHiddenUsersModel(t *testing.T) {
	users := models.DefaultProjectAuthUserModel()
	p := &models.Project{
		Schema: &models.ProjectSchema{
			Models: []*models.ModelType{
				users,
				{Name: "schedule", Fields: []*models.FieldInfo{{Identifier: "title", InputType: _const.StringInput, FieldType: "text"}}},
			},
		},
	}
	role := &models.Role{IsAdmin: true}
	cache := &models.ApplicationCache{}
	_, filtered, _, _, err := collectFilteredModelsForPublicSchema(p, cache, role)
	require.NoError(t, err)
	names := make([]string, 0, len(filtered))
	for _, m := range filtered {
		names = append(names, m.Model.Name)
	}
	require.Contains(t, names, "schedule")
	require.NotContains(t, names, "users")
}

func TestPublicSchema_noUsersListRootQuery(t *testing.T) {
	users := models.DefaultProjectAuthUserModel()
	users.Connections = []*models.ConnectionType{
		{Model: "schedule", Relation: "has_many", Type: "backward"},
	}
	schedule := &models.ModelType{
		Name: "schedule",
		Fields: []*models.FieldInfo{{Identifier: "title", InputType: _const.StringInput, FieldType: "text"}},
		Connections: []*models.ConnectionType{
			{Model: "users", Relation: "has_one", Type: "forward"},
		},
	}
	g := &GraphCtrl{
		cfg:       &models.Config{},
		gqlServer: &resolver.GraphQLServer{},
	}
	cache := &models.ApplicationCache{
		Project: &models.Project{
			ID: "p1",
			Settings: &models.ProjectSettings{
				Locals: []string{"en"},
			},
			Schema: &models.ProjectSchema{
				Models: []*models.ModelType{users, schedule},
			},
		},
		Param: &models.CommonSystemParams{
			ProjectID: "p1",
			Role:      &models.Role{IsAdmin: true},
		},
	}
	out, err := g.publicSchemaBuilder(context.Background(), cache)
	require.NoError(t, err)
	require.NotContains(t, out.RawSchemas.Queries, "usersList")
	require.NotContains(t, out.RawSchemas.Queries, "user")
	require.Contains(t, out.RawSchemas.Queries, "scheduleList")
}

package controller

import (
	"context"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/stretchr/testify/require"
)

func TestIncomingRequestIsPublicAuthOnly(t *testing.T) {
	auth := map[string]struct{}{
		"myEffectivePermissions": {},
		"loginUser":              {},
		"myTenant":               {},
	}

	require.False(t, incomingRequestIsPublicAuthOnly(nil, auth))
	require.False(t, incomingRequestIsPublicAuthOnly(&models.ApplicationCache{}, auth))

	require.True(t, incomingRequestIsPublicAuthOnly(&models.ApplicationCache{
		IncomingRequest: []*models.IncomingRequest{{
			RootFields: []string{"myEffectivePermissions"},
		}},
	}, auth))

	require.True(t, incomingRequestIsPublicAuthOnly(&models.ApplicationCache{
		IncomingRequest: []*models.IncomingRequest{{
			RootFields: []string{"myEffectivePermissions", "__typename"},
		}},
	}, auth))

	require.False(t, incomingRequestIsPublicAuthOnly(&models.ApplicationCache{
		IncomingRequest: []*models.IncomingRequest{{
			RootFields: []string{"myEffectivePermissions", "studentList"},
		}},
	}, auth))

	require.False(t, incomingRequestIsPublicAuthOnly(&models.ApplicationCache{
		IncomingRequest: []*models.IncomingRequest{{
			RootFields: []string{},
		}},
	}, auth))
}

func TestPublicSchemaBuilder_authOnlyMyEffectivePermissions(t *testing.T) {
	g := &GraphCtrl{
		cfg:       &models.Config{},
		gqlServer: &resolver.GraphQLServer{},
	}
	cache := &models.ApplicationCache{
		Project: &models.Project{
			ID: "proj-auth",
			Settings: &models.ProjectSettings{
				Locals: []string{"en"},
			},
			Schema: &models.ProjectSchema{
				Models: []*models.ModelType{
					{
						Name: "student",
						Fields: []*models.FieldInfo{
							{Identifier: "name", InputType: _const.StringInput, FieldType: "string"},
						},
					},
				},
			},
		},
		Param: &models.CommonSystemParams{
			ProjectID: "proj-auth",
			// Non-admin staff role: plan ceiling world — no ACL bypass.
			Role: &models.Role{
				ID:             "manager",
				IsAdmin:        false,
				PlanClamped:    true,
				IsProjectUser:  true,
				APIPermissions: map[string]*models.APIPermission{},
			},
		},
		// Auth-only request: extractor finds no models for myEffectivePermissions.
		IncomingRequest: []*models.IncomingRequest{{
			OperationType:     "query",
			FilteredModels:    nil,
			FilteredFunctions: nil,
			RootFields:        []string{"myEffectivePermissions"},
		}},
	}

	out, err := g.publicSchemaBuilder(context.Background(), cache)
	require.NoError(t, err)
	require.NotNil(t, out)
	require.NotNil(t, out.RawSchemas)
	require.Contains(t, out.RawSchemas.Queries, "myEffectivePermissions")
	require.Contains(t, out.RawSchemas.Queries, "loginUser")
}

func TestPublicSchemaBuilder_emptyModelsStillErrorsForModelQuery(t *testing.T) {
	g := &GraphCtrl{
		cfg:       &models.Config{},
		gqlServer: &resolver.GraphQLServer{},
	}
	cache := &models.ApplicationCache{
		Project: &models.Project{
			ID: "proj-auth",
			Settings: &models.ProjectSettings{
				Locals: []string{"en"},
			},
			Schema: &models.ProjectSchema{
				Models: []*models.ModelType{
					{
						Name: "student",
						Fields: []*models.FieldInfo{
							{Identifier: "name", InputType: _const.StringInput, FieldType: "string"},
						},
					},
				},
			},
		},
		Param: &models.CommonSystemParams{
			ProjectID: "proj-auth",
			Role: &models.Role{
				ID:             "manager",
				IsAdmin:        false,
				PlanClamped:    true,
				APIPermissions: map[string]*models.APIPermission{},
			},
		},
		IncomingRequest: []*models.IncomingRequest{{
			OperationType:  "query",
			FilteredModels: nil,
			RootFields:     []string{"studentList"},
		}},
	}

	_, err := g.publicSchemaBuilder(context.Background(), cache)
	require.Error(t, err)
	require.Contains(t, err.Error(), "query not found in schema")
}

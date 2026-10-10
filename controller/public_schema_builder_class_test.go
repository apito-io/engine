package controller

import (
	"context"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/utility"
	"github.com/stretchr/testify/require"
	"github.com/tailor-platform/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

// Protiva staff codegen sends this document. Manual class create and CSV import
// share it. If MutationType omits createClass / Class_Create_Payload is not an
// input, import fails even when manager ACL has class.create=all.
const createClassStaffDocument = `
mutation CreateClass($payload: Class_Create_Payload!, $connect: Class_Relation_Connect_Payload) {
  createClass(payload: $payload, connect: $connect, status: published) {
    id
    data { name code }
  }
}
`

func fixtureClassWithNestedRepeated() *models.ModelType {
	sectionFields := []*models.FieldInfo{
		{Identifier: "name", InputType: _const.StringInput, FieldType: "text"},
		{Identifier: "code", InputType: _const.StringInput, FieldType: "text"},
	}
	return &models.ModelType{
		Name: "class",
		Fields: []*models.FieldInfo{
			{Identifier: "name", InputType: _const.StringInput, FieldType: "text"},
			{Identifier: "code", InputType: _const.StringInput, FieldType: "text"},
			{
				Identifier: "sections",
				InputType:  _const.RepeatedInput,
				FieldType:  _const.RepeatedField,
				SubFieldInfo: sectionFields,
			},
			{
				Identifier: "divisions",
				InputType:  _const.RepeatedInput,
				FieldType:  _const.RepeatedField,
				SubFieldInfo: []*models.FieldInfo{
					{Identifier: "name", InputType: _const.StringInput, FieldType: "list"},
					{
						Identifier:   "sections",
						InputType:    _const.RepeatedInput,
						FieldType:    _const.RepeatedField,
						SubFieldInfo: sectionFields,
					},
				},
			},
		},
		Connections: []*models.ConnectionType{
			{Model: "student", Relation: "has_many"},
		},
	}
}

func TestExtractModelNames_createClass_resolvesClass(t *testing.T) {
	schema := &models.ProjectSchema{
		Models: []*models.ModelType{fixtureClassWithNestedRepeated()},
	}
	doc, err := parser.ParseQuery(&ast.Source{Input: createClassStaffDocument})
	require.NoError(t, err)
	reqs, _, err := utility.ExtractModelNames(schema, doc)
	require.NoError(t, err)
	require.Len(t, reqs, 1)
	require.Equal(t, "mutation", reqs[0].OperationType)
	require.NotEmpty(t, reqs[0].FilteredModels)
	require.Equal(t, "class", reqs[0].FilteredModels[0].Name)
}

func TestPublicSchemaBuilder_managerCreateClass_isInputMutation(t *testing.T) {
	ensureUserItemGraphQLObject()
	classModel := fixtureClassWithNestedRepeated()
	studentModel := &models.ModelType{
		Name: "student",
		Fields: []*models.FieldInfo{
			{Identifier: "name", InputType: _const.StringInput, FieldType: "text"},
		},
		Connections: []*models.ConnectionType{
			{Model: "class", Relation: "has_one"},
		},
	}
	doc, err := parser.ParseQuery(&ast.Source{Input: createClassStaffDocument})
	require.NoError(t, err)
	incoming, _, err := utility.ExtractModelNames(&models.ProjectSchema{
		Models: []*models.ModelType{classModel, studentModel},
	}, doc)
	require.NoError(t, err)

	g := &GraphCtrl{cfg: &models.Config{}, gqlServer: &resolver.GraphQLServer{}}
	cache := &models.ApplicationCache{
		Project: &models.Project{
			ID: "protiva_test",
			Settings: &models.ProjectSettings{Locals: []string{"en"}},
			Schema: &models.ProjectSchema{
				Models: []*models.ModelType{classModel, studentModel},
			},
			Roles: map[string]*models.Role{
				"manager": {
					ID: "manager",
					APIPermissions: map[string]*models.APIPermission{
						"class":   {Read: "all", Create: "all", Update: "all", Delete: "none"},
						"student": {Read: "all", Create: "all", Update: "all", Delete: "none"},
					},
				},
			},
		},
		Param: &models.CommonSystemParams{
			ProjectID: "protiva_test",
			Role: &models.Role{
				ID: "manager",
				APIPermissions: map[string]*models.APIPermission{
					"class":   {Read: "all", Create: "all", Update: "all", Delete: "none"},
					"student": {Read: "all", Create: "all", Update: "all", Delete: "none"},
				},
			},
		},
		IncomingRequest: incoming,
	}

	out, err := g.publicSchemaBuilder(context.Background(), cache)
	require.NoError(t, err)
	require.Contains(t, out.RawSchemas.Mutations, "createClass")

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
		Schema:         schema,
		RequestString:  createClassStaffDocument,
		VariableValues: map[string]interface{}{"payload": map[string]interface{}{"name": "Six", "code": "6"}},
	})
	var msgs []string
	for _, e := range result.Errors {
		msgs = append(msgs, e.Message)
	}
	for _, msg := range msgs {
		require.NotContains(t, msg, "cannot be non-input type")
		require.NotContains(t, msg, "Cannot query field \"createClass\"")
		require.NotContains(t, msg, "Unknown type \"Class_Create_Payload\"")
		require.NotContains(t, msg, "Unknown type \"Class_Relation_Connect_Payload\"")
	}
}

package controller

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/stretchr/testify/require"
	"github.com/tailor-platform/graphql"
	"github.com/tailor-platform/graphql/testutil"
)

func sortNamedObjects(arr []interface{}) {
	sort.Slice(arr, func(i, j int) bool {
		ti, _ := arr[i].(map[string]interface{})
		tj, _ := arr[j].(map[string]interface{})
		ni, _ := ti["name"].(string)
		nj, _ := tj["name"].(string)
		return ni < nj
	})
}

// stabilizeIntrospectionValue walks the introspection result and sorts every []interface{}
// whose elements are objects with a "name" field (GraphQL introspection lists are unordered).
func stabilizeIntrospectionValue(val interface{}) {
	switch x := val.(type) {
	case []interface{}:
		if len(x) > 0 {
			allNamed := true
			for _, el := range x {
				em, ok := el.(map[string]interface{})
				if !ok {
					allNamed = false
					break
				}
				if _, has := em["name"]; !has {
					allNamed = false
					break
				}
			}
			if allNamed {
				sortNamedObjects(x)
			}
			for _, el := range x {
				stabilizeIntrospectionValue(el)
			}
		}
	case map[string]interface{}:
		for _, v := range x {
			stabilizeIntrospectionValue(v)
		}
	}
}

func normalizeIntrospectionJSON(raw []byte) ([]byte, error) {
	var v map[string]interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	stabilizeIntrospectionValue(v)
	return json.MarshalIndent(v, "", "  ")
}

func runIntrospectionGolden(t *testing.T, name string, cache *models.ApplicationCache, cfg *models.Config) {
	t.Helper()
	g := &GraphCtrl{cfg: cfg, gqlServer: &resolver.GraphQLServer{}}
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
		RequestString: testutil.IntrospectionQuery,
	})
	require.Empty(t, result.Errors)
	raw, err := json.Marshal(result.Data)
	require.NoError(t, err)
	norm, err := normalizeIntrospectionJSON(raw)
	require.NoError(t, err)

	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	goldenPath := filepath.Join(dir, "testdata", "introspection_"+name+".golden.json")

	if os.Getenv("UPDATE_PUBLIC_SCHEMA_GOLDEN") == "1" {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, norm, 0o644))
		t.Logf("wrote %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "missing golden file; run with UPDATE_PUBLIC_SCHEMA_GOLDEN=1")
	require.Equal(t, string(want), string(norm))
}

func TestPublicSchemaIntrospectionGolden_oneModel_admin(t *testing.T) {
	runIntrospectionGolden(t, "one_model_admin", fixtureOneModelAdmin(), &models.Config{})
}

func TestPublicSchemaIntrospectionGolden_relations_admin(t *testing.T) {
	runIntrospectionGolden(t, "relations_admin", fixtureRelationsAdmin(), &models.Config{})
}

func TestPublicSchemaIntrospectionGolden_restricted_reader(t *testing.T) {
	runIntrospectionGolden(t, "restricted_reader", fixtureRestrictedReader(), &models.Config{})
}

func TestPublicSchemaIntrospectionGolden_roleAgnostic_matches_admin_shape(t *testing.T) {
	cache := fixtureRestrictedReader()
	cfg := &models.Config{RoleAgnosticSchemaCache: true}
	g := &GraphCtrl{cfg: cfg, gqlServer: &resolver.GraphQLServer{}}
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
		RequestString: testutil.IntrospectionQuery,
	})
	require.Empty(t, result.Errors)
	raw, err := json.Marshal(result.Data)
	require.NoError(t, err)
	norm, err := normalizeIntrospectionJSON(raw)
	require.NoError(t, err)

	_, thisFile, _, _ := runtime.Caller(0)
	dir := filepath.Dir(thisFile)
	goldenPath := filepath.Join(dir, "testdata", "introspection_role_agnostic_superset.golden.json")

	if os.Getenv("UPDATE_PUBLIC_SCHEMA_GOLDEN") == "1" {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenPath), 0o755))
		require.NoError(t, os.WriteFile(goldenPath, norm, 0o644))
		t.Logf("wrote %s", goldenPath)
		return
	}

	want, err := os.ReadFile(goldenPath)
	require.NoError(t, err, "missing golden file; run with UPDATE_PUBLIC_SCHEMA_GOLDEN=1")
	require.Equal(t, string(want), string(norm))
}

func TestPublicSchemaIntrospectionGolden_ten_models_long(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping -long in -short mode")
	}
	runIntrospectionGolden(t, "ten_models_admin", fixtureTenModelsAdmin(), &models.Config{})
}

func TestFingerprintPreConnection_roleAgnostic_excludesRole(t *testing.T) {
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
	roleA := &models.Role{IsAdmin: true}
	roleB := &models.Role{IsAdmin: false, ID: "r2"}
	a := fingerprintPreConnection(p, roleA, nil)
	b := fingerprintPreConnection(p, roleB, nil)
	require.NotEqual(t, a, b)

	ag := fingerprintPreConnection(p, nil, nil)
	bg := fingerprintPreConnection(p, nil, nil)
	require.Equal(t, ag, bg)
}

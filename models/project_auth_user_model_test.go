package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnsureProjectAuthUserModelInSchema_injectsOnce(t *testing.T) {
	schema := &ProjectSchema{Models: []*ModelType{
		{Name: "schedule", Fields: []*FieldInfo{{Identifier: "title"}}},
	}}
	require.True(t, EnsureProjectAuthUserModelInSchema(schema))
	require.Len(t, schema.Models, 2)
	require.True(t, ModelIsProjectAuthUserModel(schema.Models[1]))
	require.False(t, EnsureProjectAuthUserModelInSchema(schema))
	require.Len(t, schema.Models, 2)
}

func TestDefaultProjectAuthUserModel_safeFields(t *testing.T) {
	m := DefaultProjectAuthUserModel()
	require.Equal(t, ProjectAuthUserModelName, m.Name)
	require.True(t, m.SystemGenerated)
	ids := map[string]bool{}
	for _, f := range m.Fields {
		ids[f.Identifier] = true
	}
	require.True(t, ids["email"])
	require.True(t, ids["phone"])
	require.False(t, ids["secret"])
	require.False(t, ids["google_sub"])
}

func TestProjectAuthUserRowToDocument(t *testing.T) {
	doc, err := ProjectAuthUserRowToDocument(DefaultProjectAuthUserModel(), map[string]interface{}{
		"id":         "u1",
		"email":      "a@b.c",
		"phone":      "+1",
		"role":       "member",
		"provider":   "general",
		"status":     "active",
		"created_at": "2026-01-01T00:00:00Z",
		"updated_at": "2026-01-02T00:00:00Z",
	})
	require.NoError(t, err)
	require.Equal(t, "u1", doc.ID)
	require.Equal(t, "a@b.c", doc.Data["email"])
	require.Equal(t, "active", doc.Meta.Status)
}

func TestStripProjectAuthUserModelFromPersistedSchema(t *testing.T) {
	schema := &ProjectSchema{Models: []*ModelType{
		{Name: "book"},
		DefaultProjectAuthUserModel(),
	}}
	StripProjectAuthUserModelFromPersistedSchema(schema)
	require.Len(t, schema.Models, 1)
	require.Equal(t, "book", schema.Models[0].Name)
}

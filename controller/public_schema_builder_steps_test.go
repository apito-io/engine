package controller

import (
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestCollectFilteredModelsForPublicSchema_admin(t *testing.T) {
	p := &models.Project{
		Schema: &models.ProjectSchema{
			Models: []*models.ModelType{
				{Name: "a", Fields: []*models.FieldInfo{{Identifier: "x", InputType: _const.StringInput, FieldType: "string"}}},
			},
		},
	}
	role := &models.Role{IsAdmin: true}
	cache := &models.ApplicationCache{}
	perms, models, _, _, err := collectFilteredModelsForPublicSchema(p, cache, role)
	require.NoError(t, err)
	require.Len(t, models, 1)
	require.Contains(t, perms, "a")
}

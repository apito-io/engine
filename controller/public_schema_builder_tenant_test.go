package controller

import (
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestCollectFilteredModelsForPublicSchema_skipsTenantControlPlaneModel(t *testing.T) {
	tenant := &models.ModelType{
		Name: "tenant",
		Ext: map[string]interface{}{
			models.ExtKeyIsSystemTenantModel: true,
		},
		Fields: []*models.FieldInfo{{Identifier: "name", InputType: _const.StringInput, FieldType: "text"}},
	}
	p := &models.Project{
		Schema: &models.ProjectSchema{
			Models: []*models.ModelType{
				tenant,
				{Name: "vendor_profile", Fields: []*models.FieldInfo{{Identifier: "uid", InputType: _const.StringInput, FieldType: "text"}}},
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
	require.Contains(t, names, "vendor_profile")
	require.NotContains(t, names, "tenant")
}

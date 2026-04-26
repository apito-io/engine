package sql

import (
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestPhysicalSQLColumnForSystemRelationField(t *testing.T) {
	t.Parallel()
	require.Equal(t, "tenant_id", PhysicalSQLColumnForSystemRelationField("system_tenant_id"))
	require.Equal(t, "person_id", PhysicalSQLColumnForSystemRelationField("system_person_id"))
	require.Equal(t, "task_id", PhysicalSQLColumnForSystemRelationField("system_task_id"))
	require.Equal(t, "food_category_id", PhysicalSQLColumnForSystemRelationField("system_food_category_as_primary_id"))
	require.Equal(t, "title", PhysicalSQLColumnForSystemRelationField("title"))
	require.Equal(t, "system_other", PhysicalSQLColumnForSystemRelationField("system_other")) // no _id suffix
}

func TestRemapSyntheticSystemRelationRowKeys(t *testing.T) {
	t.Parallel()
	m := &models.ModelType{
		Fields: []*models.FieldInfo{
			{Identifier: "system_tenant_id", SystemGenerated: true, FieldType: _const.TextField, InputType: _const.StringInput},
			{Identifier: "title", SystemGenerated: false, FieldType: _const.TextField, InputType: _const.StringInput},
		},
	}
	data := map[string]interface{}{
		"system_tenant_id": "t1",
		"title":            "x",
	}
	remapSyntheticSystemRelationRowKeys(data, m)
	require.Equal(t, "t1", data["tenant_id"])
	require.Nil(t, data["system_tenant_id"])
	require.Equal(t, "x", data["title"])
}

func TestSkipDDLSyntheticSystemRelationField(t *testing.T) {
	t.Parallel()
	require.True(t, skipDDLSyntheticSystemRelationField(&models.FieldInfo{
		Identifier: "system_tenant_id", SystemGenerated: true,
	}))
	require.False(t, skipDDLSyntheticSystemRelationField(&models.FieldInfo{
		Identifier: "title", SystemGenerated: false,
	}))
	require.False(t, skipDDLSyntheticSystemRelationField(&models.FieldInfo{
		Identifier: "custom_note", SystemGenerated: true,
	}))
}

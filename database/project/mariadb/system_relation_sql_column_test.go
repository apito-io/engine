package mariadb

import (
	"testing"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestPhysicalSQLColumnForSystemRelationField(t *testing.T) {
	t.Parallel()
	require.Equal(t, "tenant_id", PhysicalSQLColumnForSystemRelationField("system_tenant_id", nil))
	require.Equal(t, "person_id", PhysicalSQLColumnForSystemRelationField("system_person_id", nil))
	require.Equal(t, "task_id", PhysicalSQLColumnForSystemRelationField("system_task_id", nil))
	require.Equal(t, "food_category_as_primary_id", PhysicalSQLColumnForSystemRelationField("system_food_category_as_primary_id", nil))
	require.Equal(t, "food_category_id", PhysicalSQLColumnForSystemRelationField("system_foodCategory_id", nil))
	require.Equal(t, "title", PhysicalSQLColumnForSystemRelationField("title", nil))
	require.Equal(t, "system_other", PhysicalSQLColumnForSystemRelationField("system_other", nil)) // no _id suffix

	mt := &models.ModelType{
		Connections: []*models.ConnectionType{
			{Model: "food_category", Relation: "has_one", Type: "forward"},
		},
	}
	require.Equal(t, "food_category_id", PhysicalSQLColumnForSystemRelationField("system_foodcategory_id", mt))
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
	}, nil))
	require.False(t, skipDDLSyntheticSystemRelationField(&models.FieldInfo{
		Identifier: "title", SystemGenerated: false,
	}, nil))
	require.False(t, skipDDLSyntheticSystemRelationField(&models.FieldInfo{
		Identifier: "custom_note", SystemGenerated: true,
	}, nil))
}

func TestRelationFKColumnNameParts(t *testing.T) {
	t.Parallel()
	require.Equal(t, "employee_id", relationFKColumnNameParts("employee", ""))
	require.Equal(t, "employee_as_waiter_id", relationFKColumnNameParts("employee", "Waiter"))
}

func TestRelationPivotTableNameParts_lexicalOrder(t *testing.T) {
	t.Parallel()
	a := relationPivotTableNameParts("stock", "draft_stock", "")
	b := relationPivotTableNameParts("draft_stock", "stock", "")
	require.Equal(t, a, b, "pivot name must not depend on argument order")
	require.Equal(t, "draft_stock_stock", a)
}

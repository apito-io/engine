package utility

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestConnectDisconnectParamBuilderPairsDuplicateTargetsByKnownAs(t *testing.T) {
	t.Parallel()

	labOrder := &models.ModelType{
		Name: "lab_order",
		Connections: []*models.ConnectionType{
			{Model: "practitioner", Relation: "has_one", Type: "forward"},
			{Model: "practitioner", Relation: "has_one", Type: "forward", KnownAs: "approval_doctor"},
			{Model: "employee", Relation: "has_one", Type: "forward"},
			{Model: "employee", Relation: "has_one", Type: "forward", KnownAs: "referrer"},
		},
	}
	practitioner := &models.ModelType{
		Name: "practitioner",
		Connections: []*models.ConnectionType{
			{Model: "lab_order", Relation: "has_many", Type: "backward"},
			{Model: "lab_order", Relation: "has_many", Type: "backward", KnownAs: "approval_doctor"},
		},
	}
	employee := &models.ModelType{
		Name: "employee",
		Connections: []*models.ConnectionType{
			{Model: "lab_order", Relation: "has_many", Type: "backward"},
			{Model: "lab_order", Relation: "has_many", Type: "backward", KnownAs: "referrer"},
		},
	}
	project := &models.Project{
		ID: "project",
		Schema: &models.ProjectSchema{
			Models: []*models.ModelType{labOrder, practitioner, employee},
		},
	}

	params, err := ConnectDisconnectParamBuilder(project, "order-1", map[string]interface{}{
		"practitioner_id":    "doctor-1",
		"approval_doctor_id": "approver-1",
		"employee_id":        "assistant-1",
		"referrer_id":        "referrer-1",
	}, labOrder)
	require.NoError(t, err)
	require.Len(t, params, 4)

	type pair struct {
		target  string
		knownAs string
	}
	got := make(map[string]pair, len(params))
	for _, param := range params {
		require.NotNil(t, param.ForwardConnectionType)
		require.NotNil(t, param.BackwardConnectionType)
		require.Equal(t, param.ForwardConnectionType.KnownAs, param.BackwardConnectionType.KnownAs)
		require.Len(t, param.ActionIDs, 1)
		got[param.ActionIDs[0]] = pair{
			target:  param.ForwardConnectionType.Model,
			knownAs: param.ForwardConnectionType.KnownAs,
		}
	}

	require.Equal(t, pair{target: "practitioner"}, got["doctor-1"])
	require.Equal(t, pair{target: "practitioner", knownAs: "approval_doctor"}, got["approver-1"])
	require.Equal(t, pair{target: "employee"}, got["assistant-1"])
	require.Equal(t, pair{target: "employee", knownAs: "referrer"}, got["referrer-1"])
}

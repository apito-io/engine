package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
)

func TestMergeQueryFilterHookSQL(t *testing.T) {
	cfg := &models.Config{
		QueryFilterHook: func(ctx context.Context, p *models.CommonSystemParams) []*models.QueryFilter {
			return []*models.QueryFilter{{
				Key:       "status",
				Condition: "==",
				Value:     "abc",
			}}
		},
	}
	param := &models.CommonSystemParams{}
	filters := map[string][]string{"AND": {}}
	err := mergeQueryFilterHookSQL(cfg, param, filters, "x", nil)
	require.NoError(t, err)
	require.Len(t, filters["AND"], 1)
	require.Contains(t, filters["AND"][0], "status")
	require.Contains(t, filters["AND"][0], "abc")
}

func TestRunDocumentPreInsertHook(t *testing.T) {
	cfg := &models.Config{
		DocumentPreInsertHook: func(ctx context.Context, p *models.CommonSystemParams, doc map[string]interface{}) error {
			doc["batch_id"] = "z1"
			return nil
		},
	}
	row := map[string]interface{}{}
	require.NoError(t, runDocumentPreInsertHook(cfg, context.Background(), &models.CommonSystemParams{}, row))
	require.Equal(t, "z1", row["batch_id"])
}

func TestSingleDocHookWhereSQL(t *testing.T) {
	cfg := &models.Config{
		QueryFilterHook: func(ctx context.Context, p *models.CommonSystemParams) []*models.QueryFilter {
			return []*models.QueryFilter{{Key: "workspace_id", Condition: "==", Value: "x7"}}
		},
	}
	s, err := singleDocHookWhereSQL(cfg, context.Background(), &models.CommonSystemParams{})
	require.NoError(t, err)
	require.True(t, strings.Contains(s, "workspace_id"))
	require.True(t, strings.Contains(s, "x7"))
}

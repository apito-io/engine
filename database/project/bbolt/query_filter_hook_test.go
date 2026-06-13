package bbolt

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/stretchr/testify/require"
)

func TestBboltDocPassesQueryHook(t *testing.T) {
	cfg := &models.Config{
		QueryFilterHook: func(ctx context.Context, p *models.CommonSystemParams) []*models.QueryFilter {
			return []*models.QueryFilter{{
				Key:       "type",
				Condition: "==",
				Value:     "want",
			}}
		},
	}
	doc := &types.DefaultDocumentStructure{Type: "want"}
	require.True(t, bboltDocPassesQueryHook(cfg, &models.CommonSystemParams{}, doc))
	doc.Type = "other"
	require.False(t, bboltDocPassesQueryHook(cfg, &models.CommonSystemParams{}, doc))
}

func TestBboltDocPassesQueryHook_DataFallback(t *testing.T) {
	cfg := &models.Config{
		QueryFilterHook: func(ctx context.Context, p *models.CommonSystemParams) []*models.QueryFilter {
			return []*models.QueryFilter{{
				Key:       "color",
				Condition: "==",
				Value:     "blue",
			}}
		},
	}
	doc := &types.DefaultDocumentStructure{Data: map[string]interface{}{"color": "blue"}}
	require.True(t, bboltDocPassesQueryHook(cfg, &models.CommonSystemParams{}, doc))
}

func TestRunDocumentPreInsertHookBbolt(t *testing.T) {
	cfg := &models.Config{
		DocumentPreInsertHook: func(ctx context.Context, p *models.CommonSystemParams, doc map[string]interface{}) error {
			doc["brand"] = "acme"
			return nil
		},
	}
	row := map[string]interface{}{}
	require.NoError(t, runDocumentPreInsertHook(cfg, context.Background(), &models.CommonSystemParams{}, row))
	require.Equal(t, "acme", row["brand"])
}

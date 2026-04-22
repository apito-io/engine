package mongo

import (
	"context"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMergeQueryFilterBSON(t *testing.T) {
	cfg := &models.Config{
		QueryFilterHook: func(ctx context.Context, p *models.CommonSystemParams) []*models.QueryFilter {
			return []*models.QueryFilter{{
				Key:       "region",
				Condition: "==",
				Value:     "eu-west",
			}}
		},
	}
	f := bson.M{"_id": "1"}
	mergeQueryFilterBSON(cfg, &models.CommonSystemParams{}, f)
	require.Equal(t, "eu-west", f["region"])
}

func TestRunDocumentPreInsertHookMongo(t *testing.T) {
	cfg := &models.Config{
		DocumentPreInsertHook: func(ctx context.Context, p *models.CommonSystemParams, doc map[string]interface{}) error {
			doc["owner_ref"] = "ins"
			return nil
		},
	}
	row := map[string]interface{}{}
	require.NoError(t, runDocumentPreInsertHook(cfg, context.Background(), &models.CommonSystemParams{}, row))
	require.Equal(t, "ins", row["owner_ref"])
}

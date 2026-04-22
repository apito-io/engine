package mongo

import (
	"context"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func effectiveCfg(cfg *models.Config, param *models.CommonSystemParams) *models.Config {
	if cfg != nil {
		return cfg
	}
	if param != nil {
		return param.RuntimeConfig
	}
	return nil
}

func hookCtx(param *models.CommonSystemParams) context.Context {
	if param != nil && param.ResolveParams != nil && param.ResolveParams.Context != nil {
		return param.ResolveParams.Context
	}
	return context.Background()
}

// mergeQueryFilterBSON merges hook filters into an existing bson.M (AND semantics for equality keys).
func mergeQueryFilterBSON(cfg *models.Config, param *models.CommonSystemParams, filter bson.M) {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.QueryFilterHook == nil || param == nil || filter == nil {
		return
	}
	ctx := hookCtx(param)
	for _, f := range cfg.QueryFilterHook(ctx, param) {
		if f == nil {
			continue
		}
		if vn := strings.TrimSpace(f.Variable); vn != "" {
			continue
		}
		key := strings.TrimSpace(f.Key)
		if key == "" {
			continue
		}
		cond := strings.TrimSpace(f.Condition)
		if cond != "=" && cond != "==" {
			continue
		}
		filter[key] = f.Value
	}
}

func runDocumentPreInsertHook(cfg *models.Config, ctx context.Context, param *models.CommonSystemParams, row map[string]interface{}) error {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.DocumentPreInsertHook == nil || param == nil || row == nil {
		return nil
	}
	hctx := ctx
	if hctx == nil {
		hctx = hookCtx(param)
	}
	return cfg.DocumentPreInsertHook(hctx, param, row)
}

func mergeHookRowIntoDocData(doc *types.DefaultDocumentStructure, row map[string]interface{}) {
	if doc == nil || len(row) == 0 {
		return
	}
	if doc.Data == nil {
		doc.Data = make(map[string]interface{}, len(row))
	}
	for k, v := range row {
		if _, exists := doc.Data[k]; !exists {
			doc.Data[k] = v
		}
	}
}

func runDocumentPreInsertDocHook(cfg *models.Config, ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) error {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.DocumentPreInsertDocHook == nil || param == nil || doc == nil {
		return nil
	}
	hctx := ctx
	if hctx == nil {
		hctx = hookCtx(param)
	}
	return cfg.DocumentPreInsertDocHook(hctx, param, doc)
}

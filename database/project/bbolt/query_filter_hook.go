package bbolt

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	apitobolt "github.com/apito-io/apitoBolt"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
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

func firstTagSegment(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(tag, ",")[0])
}

func structFieldTagKey(sf reflect.StructField) string {
	for _, t := range []string{sf.Tag.Get("bson"), sf.Tag.Get("json")} {
		if s := firstTagSegment(t); s != "" && s != "-" {
			return s
		}
	}
	return ""
}

func valueToComparableString(v reflect.Value) string {
	if !v.IsValid() {
		return ""
	}
	for v.Kind() == reflect.Interface {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	for v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}
	if !v.CanInterface() {
		return ""
	}
	switch v.Kind() {
	case reflect.String:
		return v.String()
	default:
		return fmt.Sprint(v.Interface())
	}
}

func interfaceToComparableString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

// docHookFilterStringValue resolves a filter key against top-level document fields (bson/json tags) then doc.Data.
func docHookFilterStringValue(doc *types.DefaultDocumentStructure, key string) (string, bool) {
	if doc == nil || key == "" {
		return "", false
	}
	rv := reflect.ValueOf(doc).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if sf.Name == "Data" || sf.Name == "Meta" {
			continue
		}
		if structFieldTagKey(sf) != key {
			continue
		}
		fv := rv.Field(i)
		return valueToComparableString(fv), true
	}
	if doc.Data != nil {
		if v, ok := doc.Data[key]; ok {
			return interfaceToComparableString(v), true
		}
	}
	return "", false
}

// bboltDocPassesQueryHook returns true if the document satisfies all hook equality filters (empty hook => true).
func bboltDocPassesQueryHook(cfg *models.Config, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) bool {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.QueryFilterHook == nil || param == nil || doc == nil {
		return true
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
		want := strings.TrimSpace(fmt.Sprint(f.Value))
		got, found := docHookFilterStringValue(doc, key)
		if !found {
			return false
		}
		if strings.TrimSpace(got) != want {
			return false
		}
	}
	return true
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

// bboltDeleteAllowed loads the document and verifies hook filters before delete.
func (b *BBoltDriver) bboltDeleteAllowed(ctx context.Context, param *models.CommonSystemParams, collectionName string) error {
	var doc types.DefaultDocumentStructure
	err := b.Store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)
		return col.FindByID(param.DocumentID, &doc)
	})
	if err != nil {
		return err
	}
	if !bboltDocPassesQueryHook(b.Conf, param, &doc) {
		return fmt.Errorf("document not found or access denied")
	}
	return nil
}

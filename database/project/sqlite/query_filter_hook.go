package sqlite

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types"
	"github.com/uptrace/bun"
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

// hookCtx returns GraphQL/request context for hooks when available.
func hookCtx(param *models.CommonSystemParams) context.Context {
	if param != nil && param.ResolveParams != nil && param.ResolveParams.Context != nil {
		return param.ResolveParams.Context
	}
	return context.Background()
}

// mergeQueryFilterHookSQL appends hook-produced equality predicates to filters["AND"] for raw SQL (table alias e.g. "x").
// When sqlArgs is non-nil, clauses use `?` placeholders and values are appended to sqlArgs (prepared statement path).
func mergeQueryFilterHookSQL(cfg *models.Config, param *models.CommonSystemParams, filters map[string][]string, rowAlias string, sqlArgs *[]interface{}) error {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.QueryFilterHook == nil || param == nil {
		return nil
	}
	ctx := hookCtx(param)
	for _, f := range cfg.QueryFilterHook(ctx, param) {
		if f == nil {
			continue
		}
		varName := strings.TrimSpace(f.Variable)
		if varName == "" {
			varName = rowAlias
		}
		if varName != rowAlias {
			continue
		}
		key := strings.TrimSpace(f.Key)
		if key == "" {
			continue
		}
		cond := strings.TrimSpace(f.Condition)
		if cond == "==" {
			cond = "="
		}
		if cond != "=" {
			continue
		}
		if sqlArgs != nil {
			if err := sqlEqualityClausePlaceholder(rowAlias, key, f.Value, filters, sqlArgs); err != nil {
				return err
			}
			continue
		}
		clause, err := sqlEqualityClause(rowAlias, key, f.Value)
		if err != nil {
			return err
		}
		filters["AND"] = append(filters["AND"], clause)
	}
	return nil
}

func sqlEqualityClausePlaceholder(rowAlias, key string, value interface{}, filters map[string][]string, sqlArgs *[]interface{}) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil
	}
	for _, r := range key {
		if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return fmt.Errorf("query filter hook: invalid key %q", key)
		}
	}
	filters["AND"] = append(filters["AND"], fmt.Sprintf("`%s`.`%s` = ?", rowAlias, key))
	switch v := value.(type) {
	case string:
		*sqlArgs = append(*sqlArgs, v)
	case int:
		*sqlArgs = append(*sqlArgs, v)
	case int32:
		*sqlArgs = append(*sqlArgs, int64(v))
	case int64:
		*sqlArgs = append(*sqlArgs, v)
	case float32:
		*sqlArgs = append(*sqlArgs, float64(v))
	case float64:
		*sqlArgs = append(*sqlArgs, v)
	case bool:
		*sqlArgs = append(*sqlArgs, v)
	default:
		return fmt.Errorf("query filter hook: unsupported value type for key %q", key)
	}
	return nil
}

func sqlEqualityClause(alias, key string, value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		esc := strings.ReplaceAll(v, "'", "''")
		return fmt.Sprintf("`%s`.%s = '%s'", alias, key, esc), nil
	case int, int32, int64, float32, float64, bool:
		return fmt.Sprintf("`%s`.%s = %v", alias, key, v), nil
	default:
		return "", fmt.Errorf("query filter hook: unsupported value type for key %q", key)
	}
}

// mergeQueryFilterHookAQL appends `varName.key == 'value'` style predicates (FOR/FILTER paths).
func mergeQueryFilterHookAQL(cfg *models.Config, param *models.CommonSystemParams, filters map[string][]string, varName string) error {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.QueryFilterHook == nil || param == nil {
		return nil
	}
	ctx := hookCtx(param)
	for _, f := range cfg.QueryFilterHook(ctx, param) {
		if f == nil {
			continue
		}
		vn := strings.TrimSpace(f.Variable)
		if vn == "" {
			vn = varName
		}
		if vn != varName {
			continue
		}
		key := strings.TrimSpace(f.Key)
		if key == "" {
			continue
		}
		cond := strings.TrimSpace(f.Condition)
		if cond == "==" || cond == "=" {
		} else {
			continue
		}
		clause, err := aqlEqualityClause(varName, key, f.Value)
		if err != nil {
			return err
		}
		filters["AND"] = append(filters["AND"], clause)
	}
	return nil
}

func aqlEqualityClause(varName, key string, value interface{}) (string, error) {
	switch v := value.(type) {
	case string:
		esc := strings.ReplaceAll(v, "'", "\\'")
		return fmt.Sprintf("%s.%s == '%s'", varName, key, esc), nil
	case int, int32, int64, float32, float64, bool:
		return fmt.Sprintf("%s.%s == %v", varName, key, v), nil
	default:
		return "", fmt.Errorf("query filter hook: unsupported value type for key %q", key)
	}
}

func applyBunHookWheresUpdate(cfg *models.Config, ctx context.Context, param *models.CommonSystemParams, q *bun.UpdateQuery) *bun.UpdateQuery {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.QueryFilterHook == nil || param == nil {
		return q
	}
	hctx := ctx
	if hctx == nil {
		hctx = hookCtx(param)
	}
	for _, f := range cfg.QueryFilterHook(hctx, param) {
		if f == nil || strings.TrimSpace(f.Key) == "" {
			continue
		}
		if fv := strings.TrimSpace(f.Variable); fv != "" && fv != "x" {
			continue
		}
		cond := strings.TrimSpace(f.Condition)
		if cond == "==" {
			cond = "="
		}
		if cond != "=" {
			continue
		}
		q = q.Where("? = ?", bun.Ident(strings.TrimSpace(f.Key)), f.Value)
	}
	return q
}

// runDocumentPreInsertHook runs cfg.DocumentPreInsertHook on a flat row map before INSERT.
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

func firstTagSegmentSQL(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	return strings.TrimSpace(strings.Split(tag, ",")[0])
}

// mergeDocumentTaggedFieldsIntoData copies non-empty top-level *DefaultDocumentStructure fields into the row map
// when those keys are not already present (after bson/json tag names).
func mergeDocumentTaggedFieldsIntoData(doc *types.DefaultDocumentStructure, data map[string]interface{}) {
	if doc == nil || data == nil {
		return
	}
	rv := reflect.ValueOf(doc).Elem()
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		sf := rt.Field(i)
		if sf.Name == "Data" || sf.Name == "Meta" {
			continue
		}
		tagName := firstTagSegmentSQL(sf.Tag.Get("bson"))
		if tagName == "" {
			tagName = firstTagSegmentSQL(sf.Tag.Get("json"))
		}
		if tagName == "" || tagName == "-" {
			continue
		}
		if _, exists := data[tagName]; exists {
			continue
		}
		fv := rv.Field(i)
		if !fv.CanInterface() {
			continue
		}
		if fv.Kind() == reflect.Ptr {
			if fv.IsNil() {
				continue
			}
		} else if fv.IsZero() {
			continue
		}
		data[tagName] = fv.Interface()
	}
}

func sqlModelSchemaHasFieldIdentifier(m *models.ModelType, id string) bool {
	if m == nil {
		return false
	}
	for _, f := range m.Fields {
		if f != nil && f.Identifier == id {
			return true
		}
	}
	return false
}

// StripArangoEnvelopeKeysForSQLInsert removes *DefaultDocumentStructure merge keys that SQL model DDL
// does not provision (PK is `id`; _key/_id are Arango compat; `type` mirrors the table when no field named type).
// Shared-DB SaaS only adds a physical tenant_id column (see prosql.ApplyModelPhysicalDDL); tenant_model is
// document metadata for drivers that store it in JSON/ext, not a SQL column.
func StripArangoEnvelopeKeysForSQLInsert(data map[string]interface{}, param *models.CommonSystemParams) {
	if data == nil {
		return
	}
	delete(data, "_key")
	delete(data, "_id")
	delete(data, "tenant_model")
	if param != nil && param.Model != nil {
		if v, ok := data["type"].(string); ok && strings.TrimSpace(v) == strings.TrimSpace(param.Model.Name) {
			if !sqlModelSchemaHasFieldIdentifier(param.Model, "type") {
				delete(data, "type")
			}
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

// singleDocHookWhereSQL returns " AND ..." extra predicates for raw SQL single-document reads.
// singleDocHookWhereSQLAndArgs returns extra SQL fragments with leading " AND ..." and bound args for single-doc reads.
func singleDocHookWhereSQLAndArgs(cfg *models.Config, ctx context.Context, param *models.CommonSystemParams) (string, []interface{}, error) {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.QueryFilterHook == nil || param == nil {
		return "", nil, nil
	}
	hctx := ctx
	if hctx == nil {
		hctx = hookCtx(param)
	}
	var parts []string
	var args []interface{}
	for _, f := range cfg.QueryFilterHook(hctx, param) {
		if f == nil {
			continue
		}
		vn := strings.TrimSpace(f.Variable)
		if vn != "" && vn != "x" {
			continue
		}
		cond := strings.TrimSpace(f.Condition)
		if cond == "==" {
			cond = "="
		}
		if cond != "=" {
			continue
		}
		key := strings.TrimSpace(f.Key)
		if key == "" {
			continue
		}
		for _, r := range key {
			if !(r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
				return "", nil, fmt.Errorf("query filter hook: invalid key %q", key)
			}
		}
		parts = append(parts, fmt.Sprintf("`x`.`%s` = ?", key))
		switch v := f.Value.(type) {
		case string:
			args = append(args, v)
		case int:
			args = append(args, v)
		case int32:
			args = append(args, int64(v))
		case int64:
			args = append(args, v)
		case float32:
			args = append(args, float64(v))
		case float64:
			args = append(args, v)
		case bool:
			args = append(args, v)
		default:
			return "", nil, fmt.Errorf("query filter hook: unsupported value type for key %q", key)
		}
	}
	if len(parts) == 0 {
		return "", nil, nil
	}
	return " AND " + strings.Join(parts, " AND "), args, nil
}

func singleDocHookWhereSQL(cfg *models.Config, ctx context.Context, param *models.CommonSystemParams) (string, error) {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.QueryFilterHook == nil || param == nil {
		return "", nil
	}
	hctx := ctx
	if hctx == nil {
		hctx = hookCtx(param)
	}
	var parts []string
	for _, f := range cfg.QueryFilterHook(hctx, param) {
		if f == nil {
			continue
		}
		vn := strings.TrimSpace(f.Variable)
		if vn != "" && vn != "x" {
			continue
		}
		cond := strings.TrimSpace(f.Condition)
		if cond == "==" {
			cond = "="
		}
		if cond != "=" {
			continue
		}
		clause, err := sqlEqualityClause("x", strings.TrimSpace(f.Key), f.Value)
		if err != nil {
			return "", err
		}
		parts = append(parts, clause)
	}
	if len(parts) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(parts, " AND "), nil
}

func applyBunHookWheresDelete(cfg *models.Config, ctx context.Context, param *models.CommonSystemParams, q *bun.DeleteQuery) *bun.DeleteQuery {
	cfg = effectiveCfg(cfg, param)
	if cfg == nil || cfg.QueryFilterHook == nil || param == nil {
		return q
	}
	hctx := ctx
	if hctx == nil {
		hctx = hookCtx(param)
	}
	for _, f := range cfg.QueryFilterHook(hctx, param) {
		if f == nil || strings.TrimSpace(f.Key) == "" {
			continue
		}
		if fv := strings.TrimSpace(f.Variable); fv != "" && fv != "x" {
			continue
		}
		cond := strings.TrimSpace(f.Condition)
		if cond == "==" {
			cond = "="
		}
		if cond != "=" {
			continue
		}
		q = q.Where("? = ?", bun.Ident(strings.TrimSpace(f.Key)), f.Value)
	}
	return q
}

package models

import (
	"context"
	"strings"
)

type sharedSaaSIndexKey struct{}

// WithSharedSaaSIndex marks DDL that must prefix composite unique indexes with tenant_id.
// Separate-database SaaS and general projects leave this unset.
func WithSharedSaaSIndex(ctx context.Context, on bool) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if !on {
		return ctx
	}
	return context.WithValue(ctx, sharedSaaSIndexKey{}, true)
}

// SharedSaaSIndexFromContext reports whether composite unique indexes should include tenant_id.
func SharedSaaSIndexFromContext(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	on, _ := ctx.Value(sharedSaaSIndexKey{}).(bool)
	return on
}

// SharedSaaSIndexFromParam is true for a SaaS project that stores every tenant in one database.
// Separate-database SaaS is false. Kind is project_kind=saas, or the legacy numeric host key.
func SharedSaaSIndexFromParam(p *CommonSystemParams) bool {
	if p == nil || p.Ext == nil || ParamPerTenantSeparateDB(p) {
		return false
	}
	return ProjectKindIsSaaS(p.Ext)
}

// ProjectKindIsSaaS reports a shared-database SaaS project from ext.
// project_kind "saas" is the public key. The legacy numeric key is still read.
func ProjectKindIsSaaS(ext map[string]interface{}) bool {
	if ext == nil {
		return false
	}
	switch v := ext["project_kind"].(type) {
	case string:
		s := strings.ToLower(strings.TrimSpace(v))
		if s == "saas" || s == "1" {
			return true
		}
	}
	if extIntOne(ext["project_kind"]) || extIntOne(ext["pro_project_type"]) {
		return true
	}
	return false
}

func extIntOne(v interface{}) bool {
	switch n := v.(type) {
	case int32:
		return n == 1
	case int:
		return n == 1
	case int64:
		return n == 1
	case float64:
		return int(n) == 1
	default:
		return false
	}
}

// CompositeUniqueIndexName is idx_<table>_<key>.
func CompositeUniqueIndexName(table, keyName string) string {
	return "idx_" + strings.TrimSpace(table) + "_" + strings.TrimSpace(keyName)
}

// OneToOneUniqueIndexName is the single-column unique created for has_one/has_one.
func OneToOneUniqueIndexName(table, fkColumn string) string {
	return "idx_" + strings.TrimSpace(table) + "_" + strings.TrimSpace(fkColumn) + "_unique"
}

// UniqueIndexDrop names a composite unique index publish should remove.
type UniqueIndexDrop struct {
	Table string
	Name  string
}

// UniqueKeyIndexColumns prefixes tenant_id for shared-database SaaS when it is not already listed.
func UniqueKeyIndexColumns(sharedSaaS bool, columns []string) []string {
	if !sharedSaaS {
		return columns
	}
	for _, c := range columns {
		if strings.EqualFold(strings.TrimSpace(c), "tenant_id") {
			return columns
		}
	}
	out := make([]string, 0, len(columns)+1)
	out = append(out, "tenant_id")
	out = append(out, columns...)
	return out
}

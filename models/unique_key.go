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
// pro_project_type 1 is SaaS (engine proconst). Separate-database SaaS is false.
func SharedSaaSIndexFromParam(p *CommonSystemParams) bool {
	if p == nil || p.Ext == nil || ParamPerTenantSeparateDB(p) {
		return false
	}
	switch v := p.Ext["pro_project_type"].(type) {
	case int32:
		return v == 1
	case int:
		return v == 1
	case int64:
		return v == 1
	case float64:
		return v == 1
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

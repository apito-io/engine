package utility

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/vektah/gqlparser/v2/ast"
)

// Typed context keys for GraphQL execution (avoid string collisions and vet warnings).
type graphqlCtxKey int

const (
	ctxKeyApplicationCache graphqlCtxKey = iota + 1
	ctxKeySelectionSet
	ctxKeyRelationMeta
)

// WithApplicationCache attaches the application cache to ctx.
func WithApplicationCache(ctx context.Context, c *models.ApplicationCache) context.Context {
	return context.WithValue(ctx, ctxKeyApplicationCache, c)
}

// ApplicationCacheFromContext returns the cache set by WithApplicationCache.
func ApplicationCacheFromContext(ctx context.Context) (*models.ApplicationCache, bool) {
	v, ok := ctx.Value(ctxKeyApplicationCache).(*models.ApplicationCache)
	return v, ok
}

// WithSelectionSet attaches the root GraphQL selection set (for relation resolution).
func WithSelectionSet(ctx context.Context, sel ast.SelectionSet) context.Context {
	return context.WithValue(ctx, ctxKeySelectionSet, sel)
}

// SelectionSetFromContext returns the selection set from ctx.
func SelectionSetFromContext(ctx context.Context) (ast.SelectionSet, bool) {
	v, ok := ctx.Value(ctxKeySelectionSet).(ast.SelectionSet)
	return v, ok
}

// WithRelationMeta attaches relation metadata for dataloaders (replaces string key "relation_meta").
func WithRelationMeta(ctx context.Context, meta map[string]interface{}) context.Context {
	return context.WithValue(ctx, ctxKeyRelationMeta, meta)
}

// RelationMetaFromContext returns relation metadata from ctx.
func RelationMetaFromContext(ctx context.Context) (map[string]interface{}, bool) {
	v, ok := ctx.Value(ctxKeyRelationMeta).(map[string]interface{})
	return v, ok
}

// LegacyApplicationCache returns cache from typed key or legacy string "cache".
func LegacyApplicationCache(ctx context.Context) (*models.ApplicationCache, bool) {
	if c, ok := ApplicationCacheFromContext(ctx); ok {
		return c, true
	}
	if v := ctx.Value("cache"); v != nil {
		if c, ok := v.(*models.ApplicationCache); ok {
			return c, true
		}
	}
	return nil, false
}

// LegacySelectionSet returns selection set from typed key or legacy "selectionSet".
func LegacySelectionSet(ctx context.Context) (ast.SelectionSet, bool) {
	if s, ok := SelectionSetFromContext(ctx); ok {
		return s, true
	}
	if v := ctx.Value("selectionSet"); v != nil {
		if s, ok := v.(ast.SelectionSet); ok {
			return s, true
		}
	}
	return nil, false
}

// LegacyRelationMeta returns relation metadata from typed key or legacy "relation_meta".
func LegacyRelationMeta(ctx context.Context) (map[string]interface{}, bool) {
	if m, ok := RelationMetaFromContext(ctx); ok {
		return m, true
	}
	if v := ctx.Value("relation_meta"); v != nil {
		if m, ok := v.(map[string]interface{}); ok {
			return m, true
		}
	}
	return nil, false
}

package models

import (
	"context"
	"fmt"
)

// Well-known Ext keys shared with a host binary (this module must not import the host).
const (
	ExtKeyIsCommonModel             = "is_common_model"
	ExtKeyPerTenantSeparateDatabase = "per_tenant_separate_db"
)

// SkipConnectionRoutingContextKey forces GetProjectDriver onto the base project DB.
const SkipConnectionRoutingContextKey = "skip_connection_routing"

type schemaDDLTargetKey struct{}

// WithSchemaDDLTenantTarget marks schema DDL as running against a tenant-scoped connection.
func WithSchemaDDLTenantTarget(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, schemaDDLTargetKey{}, true)
}

// IsSchemaDDLTenantTarget is true inside SchemaIterateHook tenant driver callbacks.
func IsSchemaDDLTenantTarget(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(schemaDDLTargetKey{}).(bool)
	return v
}

// WithBaseProjectDB skips per-tenant connection routing so the caller hits the base project DB.
func WithBaseProjectDB(ctx context.Context) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, SkipConnectionRoutingContextKey, true)
}

// ModelIsCommon reads Ext["is_common_model"] without importing the pro package.
func ModelIsCommon(m *ModelType) bool {
	if m == nil || m.Ext == nil {
		return false
	}
	v, ok := m.Ext[ExtKeyIsCommonModel].(bool)
	return ok && v
}

// ModelIsTenantCatalogue reads Ext["is_tenant_model"].
func ModelIsTenantCatalogue(m *ModelType) bool {
	if m == nil || m.Ext == nil {
		return false
	}
	v, ok := m.Ext[ExtKeyIsTenantModel].(bool)
	return ok && v
}

// ParamPerTenantSeparateDB reads Ext["per_tenant_separate_db"] on a request param.
func ParamPerTenantSeparateDB(p *CommonSystemParams) bool {
	if p == nil || p.Ext == nil {
		return false
	}
	v, ok := p.Ext[ExtKeyPerTenantSeparateDatabase].(bool)
	return ok && v
}

// UseBaseProjectDB is true when this request's target model is a common model
// on a per-tenant-separate-database project — documents live in the base DB.
func UseBaseProjectDB(p *CommonSystemParams) bool {
	if p == nil {
		return false
	}
	return ParamPerTenantSeparateDB(p) && ModelIsCommon(p.Model)
}

// RelationLoaderUsesBaseDB is true when a nested relation JOIN must run on the
// base project DB. Common→common FKs live only on base; tenant tables are
// id-only shadows and cannot satisfy SELECT x.name / x.generic_id. Tenant→common
// stays on the tenant connection (FK on the tenant row, then hydrate).
func RelationLoaderUsesBaseDB(param *CommonSystemParams, parent, related *ModelType) bool {
	if param == nil || parent == nil || related == nil {
		return false
	}
	return ParamPerTenantSeparateDB(param) && ModelIsCommon(parent) && ModelIsCommon(related)
}

// SkipCommonModelShadowFieldDDL is true when tenant-DB DDL must leave a common
// model as an id-only shadow table (no user field columns).
func SkipCommonModelShadowFieldDDL(ctx context.Context, model *ModelType) bool {
	return IsSchemaDDLTenantTarget(ctx) && ModelIsCommon(model)
}

// IsBaseProjectDB is true when GetProjectDriver was routed with skip_connection_routing.
func IsBaseProjectDB(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, _ := ctx.Value(SkipConnectionRoutingContextKey).(bool)
	return v
}

// SkipRelationArtifactsOnConnection skips FK/pivot DDL that does not belong on
// this physical database. The base project DB only stores common↔common
// relations; tenant DBs skip common↔common (those FKs live on base).
func SkipRelationArtifactsOnConnection(ctx context.Context, from, to *ModelType) bool {
	if from == nil || to == nil {
		return false
	}
	bothCommon := ModelIsCommon(from) && ModelIsCommon(to)
	if IsBaseProjectDB(ctx) {
		return !bothCommon
	}
	if IsSchemaDDLTenantTarget(ctx) && bothCommon {
		return true
	}
	return false
}

// ProjectDBContext returns a context that routes GetProjectDriver to the base
// project DB when the param's model is common on a per-tenant-separate project.
func ProjectDBContext(ctx context.Context, param *CommonSystemParams) context.Context {
	if UseBaseProjectDB(param) {
		return WithBaseProjectDB(ctx)
	}
	return ctx
}

// ValidateCommonTenantRelation rejects a common model holding an FK to a tenant-scoped model.
func ValidateCommonTenantRelation(fromModel, toModel *ModelType, fromRel, toRel string) error {
	if fromModel == nil || toModel == nil {
		return nil
	}
	fromCommon := ModelIsCommon(fromModel)
	toCommon := ModelIsCommon(toModel)
	if fromCommon == toCommon {
		return nil
	}
	if fromRel == "has_one" && fromCommon && !toCommon {
		return fmt.Errorf("common model %q cannot hold a foreign key to tenant-scoped %q; put has_one on the tenant-scoped model so the relation lives in the tenant database", fromModel.Name, toModel.Name)
	}
	if toRel == "has_one" && toCommon && !fromCommon {
		return fmt.Errorf("common model %q cannot hold a foreign key to tenant-scoped %q; put has_one on the tenant-scoped model so the relation lives in the tenant database", toModel.Name, fromModel.Name)
	}
	return nil
}

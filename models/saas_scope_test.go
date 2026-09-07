package models

import (
	"context"
	"testing"
)

func TestUseBaseProjectDB(t *testing.T) {
	common := &ModelType{Name: "medicine", Ext: map[string]interface{}{ExtKeyIsCommonModel: true}}
	stock := &ModelType{Name: "stock"}
	p := &CommonSystemParams{
		Model: common,
		Ext:   map[string]interface{}{ExtKeyPerTenantSeparateDatabase: true},
	}
	if !UseBaseProjectDB(p) {
		t.Fatal("common + per-tenant separate must use base DB")
	}
	p.Model = stock
	if UseBaseProjectDB(p) {
		t.Fatal("tenant-scoped model must not use base DB")
	}
	p.Model = common
	p.Ext[ExtKeyPerTenantSeparateDatabase] = false
	if UseBaseProjectDB(p) {
		t.Fatal("shared-DB must not use the per-tenant base routing path")
	}
}

func TestRelationLoaderUsesBaseDB(t *testing.T) {
	common := &ModelType{Name: "medicine", Ext: map[string]interface{}{ExtKeyIsCommonModel: true}}
	generic := &ModelType{Name: "generic", Ext: map[string]interface{}{ExtKeyIsCommonModel: true}}
	stock := &ModelType{Name: "stock"}
	p := &CommonSystemParams{Ext: map[string]interface{}{ExtKeyPerTenantSeparateDatabase: true}}
	if !RelationLoaderUsesBaseDB(p, common, generic) {
		t.Fatal("common→common nested reads must use the base DB")
	}
	if RelationLoaderUsesBaseDB(p, stock, common) {
		t.Fatal("tenant→common JOINs stay on the tenant DB")
	}
	p.Ext[ExtKeyPerTenantSeparateDatabase] = false
	if RelationLoaderUsesBaseDB(p, common, generic) {
		t.Fatal("shared-DB must not force base routing")
	}
}

func TestSkipCommonModelShadowFieldDDL(t *testing.T) {
	common := &ModelType{Name: "medicine", Ext: map[string]interface{}{ExtKeyIsCommonModel: true}}
	ctx := WithSchemaDDLTenantTarget(context.Background())
	if !SkipCommonModelShadowFieldDDL(ctx, common) {
		t.Fatal("tenant-target DDL for common models must skip field columns")
	}
	if SkipCommonModelShadowFieldDDL(context.Background(), common) {
		t.Fatal("base DDL for common models must add field columns")
	}
	if SkipCommonModelShadowFieldDDL(ctx, &ModelType{Name: "stock"}) {
		t.Fatal("tenant-scoped models keep full columns on tenant DBs")
	}
}

func TestSkipRelationArtifactsOnConnection(t *testing.T) {
	commonA := &ModelType{Name: "medicine", Ext: map[string]interface{}{ExtKeyIsCommonModel: true}}
	commonB := &ModelType{Name: "generic", Ext: map[string]interface{}{ExtKeyIsCommonModel: true}}
	stock := &ModelType{Name: "stock"}
	reagent := &ModelType{Name: "reagent"}
	base := WithBaseProjectDB(context.Background())
	tenantDDL := WithSchemaDDLTenantTarget(context.Background())

	if SkipRelationArtifactsOnConnection(base, commonA, commonB) {
		t.Fatal("common↔common FKs belong on the base DB")
	}
	if !SkipRelationArtifactsOnConnection(base, stock, reagent) {
		t.Fatal("tenant↔tenant FKs must not be created on the base DB")
	}
	if !SkipRelationArtifactsOnConnection(base, stock, commonA) {
		t.Fatal("tenant→common FKs live on the tenant DB, not base")
	}
	if !SkipRelationArtifactsOnConnection(tenantDDL, commonA, commonB) {
		t.Fatal("tenant DBs must not get common↔common FK DDL")
	}
	if SkipRelationArtifactsOnConnection(tenantDDL, stock, commonA) {
		t.Fatal("tenant→common FKs belong on the tenant DB")
	}
	if SkipRelationArtifactsOnConnection(context.Background(), stock, reagent) {
		t.Fatal("shared or tenant data connections still apply tenant↔tenant FKs")
	}
}

func TestValidateCommonTenantRelation(t *testing.T) {
	common := &ModelType{Name: "medicine", Ext: map[string]interface{}{ExtKeyIsCommonModel: true}}
	stock := &ModelType{Name: "stock"}
	if err := ValidateCommonTenantRelation(stock, common, "has_one", "has_many"); err != nil {
		t.Fatalf("tenant has_one common is allowed: %v", err)
	}
	if err := ValidateCommonTenantRelation(common, stock, "has_one", "has_many"); err == nil {
		t.Fatal("common has_one tenant must be rejected")
	}
	if err := ValidateCommonTenantRelation(common, stock, "has_many", "has_many"); err != nil {
		t.Fatalf("mixed M:N is tenant-local and allowed: %v", err)
	}
}

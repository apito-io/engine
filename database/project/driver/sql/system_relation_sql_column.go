package sql

import (
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

const systemRelationFieldCoreAs = "_as_"

func collapseSyntheticRelationCore(s string) string {
	return strings.ReplaceAll(strings.ToLower(strings.TrimSpace(s)), "_", "")
}

func resolvePhysicalFKFromSchemaSyntheticCore(core string, modelType *models.ModelType) string {
	if modelType == nil || strings.TrimSpace(core) == "" {
		return ""
	}
	want := collapseSyntheticRelationCore(core)
	for _, conn := range modelType.Connections {
		if conn == nil || conn.Relation != "has_one" {
			continue
		}
		syn := utility.SyntheticSystemRelationFieldIdentifier(conn.Model, conn.KnownAs)
		if syn == "" {
			continue
		}
		const pfx, sfx = "system_", "_id"
		if !strings.HasPrefix(syn, pfx) || !strings.HasSuffix(syn, sfx) {
			continue
		}
		synCore := strings.TrimSuffix(strings.TrimPrefix(syn, pfx), sfx)
		if collapseSyntheticRelationCore(synCore) == want {
			return utility.PhysicalSQLTableName(conn.Model) + "_id"
		}
	}
	return ""
}

// PhysicalSQLColumnForSystemRelationField maps GraphQL/schema synthetic relation keys
// (system_<model>_id or system_<model>_as_<known>_id) to physical FK column names created
// by AddRelationFields. Pass modelType when available so legacy collapsed ids (e.g.
// system_foodcategory_id vs system_food_category_id) resolve to the same SQL column.
func PhysicalSQLColumnForSystemRelationField(identifier string, modelType *models.ModelType) string {
	id := strings.TrimSpace(identifier)
	id = utility.CanonicalSystemRelationFieldIdentifier(id)
	const pfx, sfx = "system_", "_id"
	if !strings.HasPrefix(id, pfx) || !strings.HasSuffix(id, sfx) {
		return id
	}
	core := strings.TrimSuffix(strings.TrimPrefix(id, pfx), sfx)
	if core == "" {
		return id
	}
	if modelType != nil {
		if resolved := resolvePhysicalFKFromSchemaSyntheticCore(core, modelType); resolved != "" {
			return resolved
		}
	}
	// SQL DDL uses the referenced model segment only (known_as does not change the column name today).
	slug := core
	if idx := strings.Index(core, systemRelationFieldCoreAs); idx >= 0 {
		slug = core[:idx]
	}
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return id
	}
	physSlug := utility.PhysicalSQLTableName(slug)
	if physSlug == "" {
		return slug + "_id"
	}
	return physSlug + "_id"
}

// remapSyntheticSystemRelationRowKeys copies values from system_*_id keys onto physical FK
// column keys for SQL INSERT/UPDATE row maps when the model schema declares those synthetic fields.
func remapSyntheticSystemRelationRowKeys(data map[string]interface{}, model *models.ModelType) {
	if data == nil || model == nil {
		return
	}
	for _, f := range model.Fields {
		if f == nil || !f.SystemGenerated {
			continue
		}
		gql := strings.TrimSpace(f.Identifier)
		if gql == "" {
			continue
		}
		phys := PhysicalSQLColumnForSystemRelationField(gql, model)
		if phys == gql {
			continue
		}
		if v, ok := data[gql]; ok {
			if _, taken := data[phys]; !taken {
				data[phys] = v
			}
			delete(data, gql)
		}
	}
}

// skipDDLSyntheticSystemRelationField avoids ALTER ADD for hidden relation keys that map to
// an existing FK column (AddRelationFields / SaaS tenant_id) so we do not duplicate columns.
func skipDDLSyntheticSystemRelationField(f *models.FieldInfo, modelType *models.ModelType) bool {
	if f == nil || !f.SystemGenerated {
		return false
	}
	gql := strings.TrimSpace(f.Identifier)
	phys := PhysicalSQLColumnForSystemRelationField(gql, modelType)
	return phys != gql
}

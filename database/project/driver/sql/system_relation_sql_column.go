package sql

import (
	"strings"

	"github.com/apito-io/engine/models"
)

const systemRelationFieldCoreAs = "_as_"

// PhysicalSQLColumnForSystemRelationField maps GraphQL/schema synthetic relation keys
// (system_<model>_id or system_<model>_as_<knownAs>_id) to physical FK column names created
// by AddRelationFields (<singular_model>_id). Non-matching identifiers are returned unchanged.
func PhysicalSQLColumnForSystemRelationField(identifier string) string {
	id := strings.TrimSpace(identifier)
	const pfx, sfx = "system_", "_id"
	if !strings.HasPrefix(id, pfx) || !strings.HasSuffix(id, sfx) {
		return id
	}
	core := strings.TrimSuffix(strings.TrimPrefix(id, pfx), sfx)
	if core == "" {
		return id
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
	// Match AddRelationFields: FK columns are `<from.Model>_id` using the same model id string
	// stored on the connection (not re-singularized), so the slug from system_* must map 1:1.
	return slug + "_id"
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
		phys := PhysicalSQLColumnForSystemRelationField(gql)
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
func skipDDLSyntheticSystemRelationField(f *models.FieldInfo) bool {
	if f == nil || !f.SystemGenerated {
		return false
	}
	gql := strings.TrimSpace(f.Identifier)
	phys := PhysicalSQLColumnForSystemRelationField(gql)
	return phys != gql
}

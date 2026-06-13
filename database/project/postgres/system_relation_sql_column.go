package postgres

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
			return relationFKColumnName(conn)
		}
	}
	return ""
}

func relationFKColumnNameParts(model, knownAs string) string {
	modelSlug := strings.TrimSpace(utility.PhysicalSQLTableName(model))
	if modelSlug == "" {
		return ""
	}
	knownSlug := strings.TrimSpace(utility.PhysicalSQLTableName(knownAs))
	if knownSlug == "" {
		return modelSlug + "_id"
	}
	return modelSlug + "_as_" + knownSlug + "_id"
}

func relationFKColumnName(conn *models.ConnectionType) string {
	if conn == nil {
		return ""
	}
	return relationFKColumnNameParts(conn.Model, conn.KnownAs)
}

func relationKnownAs(conns ...*models.ConnectionType) string {
	for _, conn := range conns {
		if conn == nil {
			continue
		}
		if known := strings.TrimSpace(conn.KnownAs); known != "" {
			return known
		}
	}
	return ""
}

func relationFKColumnNameForModel(model string, conns ...*models.ConnectionType) string {
	return relationFKColumnNameParts(model, relationKnownAs(conns...))
}

// sortedPhysicalPair returns the two physical SQL table slugs for modelA and modelB in
// lexicographic order (a <= b). Used so pivot table names and DDL match list/connect/query
// regardless of which endpoint is “forward” vs “backward” or schema iteration order.
func sortedPhysicalPair(modelA, modelB string) (string, string) {
	a := strings.TrimSpace(utility.PhysicalSQLTableName(modelA))
	b := strings.TrimSpace(utility.PhysicalSQLTableName(modelB))
	if a == "" || b == "" {
		return a, b
	}
	if a > b {
		return b, a
	}
	return a, b
}

func relationPivotTableNameParts(fromModel, toModel, knownAs string) string {
	a, b := sortedPhysicalPair(fromModel, toModel)
	if a == "" || b == "" {
		return ""
	}
	base := a + "_" + b
	known := strings.TrimSpace(utility.PhysicalSQLTableName(knownAs))
	if known == "" {
		return base
	}
	return base + "_as_" + known
}

func relationPivotTableName(from, to *models.ConnectionType) string {
	if from == nil || to == nil {
		return ""
	}
	knownAs := strings.TrimSpace(from.KnownAs)
	if knownAs == "" {
		knownAs = strings.TrimSpace(to.KnownAs)
	}
	return relationPivotTableNameParts(from.Model, to.Model, knownAs)
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
	slug := strings.TrimSpace(core)
	if slug == "" {
		return id
	}
	if idx := strings.Index(slug, systemRelationFieldCoreAs); idx >= 0 {
		modelSlug := strings.TrimSpace(slug[:idx])
		knownAsSlug := strings.TrimSpace(slug[idx+len(systemRelationFieldCoreAs):])
		if col := relationFKColumnNameParts(modelSlug, knownAsSlug); col != "" {
			return col
		}
	}
	if col := relationFKColumnNameParts(slug, ""); col != "" {
		return col
	}
	return id
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

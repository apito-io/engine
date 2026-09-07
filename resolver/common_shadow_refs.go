package resolver

import (
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
)

type commonShadowRefSource struct {
	Table  string
	Column string
}

func commonModelShadowRefSources(schemaModels []*models.ModelType, common *models.ModelType) []commonShadowRefSource {
	if common == nil || strings.TrimSpace(common.Name) == "" {
		return nil
	}
	var out []commonShadowRefSource
	seen := map[string]struct{}{}
	add := func(table, column string) {
		table = strings.TrimSpace(table)
		column = strings.TrimSpace(column)
		if table == "" || column == "" {
			return
		}
		key := strings.ToLower(table) + "\x00" + strings.ToLower(column)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, commonShadowRefSource{Table: table, Column: column})
	}
	commonName := common.Name
	commonPhys := utility.PhysicalSQLTableName(commonName)
	for _, m := range schemaModels {
		if m == nil || models.ModelIsCommon(m) {
			continue
		}
		for _, c := range m.Connections {
			if c == nil {
				continue
			}
			if utility.PhysicalSQLTableName(c.Model) != commonPhys && !strings.EqualFold(c.Model, commonName) {
				continue
			}
			switch strings.ToLower(strings.TrimSpace(c.Relation)) {
			case "has_one":
				add(utility.PhysicalSQLTableName(m.Name), commonShadowFKColumn(c))
			case "has_many":
				add(commonShadowPivotTable(m.Name, commonName, c.KnownAs), commonPhys+"_id")
			}
		}
	}
	return out
}

func commonShadowFKColumn(conn *models.ConnectionType) string {
	if conn == nil {
		return ""
	}
	modelSlug := strings.TrimSpace(utility.PhysicalSQLTableName(conn.Model))
	if modelSlug == "" {
		return ""
	}
	knownSlug := strings.TrimSpace(utility.PhysicalSQLTableName(conn.KnownAs))
	if knownSlug == "" {
		return modelSlug + "_id"
	}
	return modelSlug + "_as_" + knownSlug + "_id"
}

func commonShadowPivotTable(fromModel, toModel, knownAs string) string {
	a := strings.TrimSpace(utility.PhysicalSQLTableName(fromModel))
	b := strings.TrimSpace(utility.PhysicalSQLTableName(toModel))
	if a == "" || b == "" {
		return ""
	}
	if a > b {
		a, b = b, a
	}
	base := a + "_" + b
	known := strings.TrimSpace(utility.PhysicalSQLTableName(knownAs))
	if known == "" {
		return base
	}
	return base + "_as_" + known
}

func commonDocIsSoftDeleted(doc *types.DefaultDocumentStructure) bool {
	return doc != nil && doc.Meta != nil && strings.EqualFold(strings.TrimSpace(doc.Meta.Status), "deleted")
}

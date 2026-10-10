package utility

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/apito-io/engine/models"
)

var uniqueKeyNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,48}$`)
var uniqueKeyColumnRe = regexp.MustCompile(`^[a-z][a-z0-9_]+$`)

// RelationFKColumn is the physical foreign-key column AddRelationFields creates for a has_one peer.
func RelationFKColumn(model, knownAs string) string {
	slug := PhysicalSQLTableName(model)
	if slug == "" {
		return ""
	}
	known := strings.TrimSpace(knownAs)
	if known == "" {
		return slug + "_id"
	}
	knownSlug := PhysicalSQLTableName(known)
	if knownSlug == "" {
		return slug + "_id"
	}
	return slug + "_as_" + knownSlug + "_id"
}

// ValidateUniqueKey checks a composite unique key against one model's scalar fields and has_one foreign keys.
func ValidateUniqueKey(m *models.ModelType, name string, columns []string) (*models.UniqueKey, error) {
	if m == nil || strings.TrimSpace(m.Name) == "" {
		return nil, fmt.Errorf("model is required")
	}
	name = strings.ToLower(strings.TrimSpace(name))
	if !uniqueKeyNameRe.MatchString(name) {
		return nil, fmt.Errorf("unique key name %q must be a lowercase identifier", name)
	}
	if len(columns) < 2 {
		return nil, fmt.Errorf("unique key %q needs at least two columns", name)
	}
	seen := map[string]struct{}{}
	clean := make([]string, 0, len(columns))
	for _, raw := range columns {
		col := strings.TrimSpace(raw)
		if !uniqueKeyColumnRe.MatchString(col) {
			return nil, fmt.Errorf("unique key column %q is not a physical column name", raw)
		}
		if _, ok := seen[col]; ok {
			return nil, fmt.Errorf("unique key %q lists column %q twice", name, col)
		}
		if !modelHasUniqueKeyColumn(m, col) {
			return nil, fmt.Errorf("model %q has no column %q (use a field identifier or a has_one foreign key such as variant_id)", m.Name, col)
		}
		seen[col] = struct{}{}
		clean = append(clean, col)
	}
	return &models.UniqueKey{Name: name, Columns: clean}, nil
}

func modelHasUniqueKeyColumn(m *models.ModelType, col string) bool {
	if m == nil {
		return false
	}
	for _, f := range m.Fields {
		if f != nil && f.Identifier == col && strings.TrimSpace(f.ParentField) == "" {
			return true
		}
	}
	for _, c := range m.Connections {
		if c == nil || c.Relation != "has_one" {
			continue
		}
		if RelationFKColumn(c.Model, c.KnownAs) == col {
			return true
		}
	}
	return false
}

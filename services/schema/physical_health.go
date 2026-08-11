package schema

import (
	"sort"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

// ModelPhysicalHealth is the read-only comparison of logical schema fields
// against physical table columns (no DDL).
type ModelPhysicalHealth struct {
	ModelName        string   `json:"model_name"`
	TableExists      bool     `json:"table_exists"`
	PhysicalColumns  []string `json:"physical_columns"`
	ExpectedColumns  []string `json:"expected_columns"`
	MissingColumns   []string `json:"missing_columns"`
	ExtraColumns     []string `json:"extra_columns"`
	IsCommonModel    bool     `json:"is_common_model"`
	Warnings         []string `json:"warnings"`
}

// ExpectedPhysicalColumns returns top-level SQL columns implied by the model
// schema (id + field identifiers / locale expansions). Nested object/repeated
// children are not separate columns (parent is JSON).
func ExpectedPhysicalColumns(model *models.ModelType) []string {
	if model == nil {
		return []string{"id"}
	}
	seen := map[string]struct{}{"id": {}}
	out := []string{"id"}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, name)
	}
	for _, f := range model.Fields {
		if f == nil || strings.TrimSpace(f.Identifier) == "" {
			continue
		}
		// Only top-level fields become columns; nested live inside JSON parents.
		if strings.TrimSpace(f.ParentField) != "" {
			continue
		}
		if !fieldMapsToPhysicalColumn(f) {
			continue
		}
		if f.Validation != nil && len(f.Validation.Locals) > 0 {
			for _, local := range f.Validation.Locals {
				local = strings.TrimSpace(local)
				if local == "" || local == "en" {
					add(f.Identifier)
					continue
				}
				add(f.Identifier + "_" + local)
			}
			continue
		}
		add(f.Identifier)
	}
	return normalizeColumnList(out)
}

func fieldMapsToPhysicalColumn(f *models.FieldInfo) bool {
	if f == nil {
		return false
	}
	if f.InputType == "geo" {
		return false
	}
	switch f.FieldType {
	case _const.TextField, _const.MultilineField, _const.DateField, _const.BooleanField,
		_const.MediaField, _const.NumberField, _const.ListField,
		_const.RepeatedField, _const.ObjectField:
		return true
	default:
		return false
	}
}

// ModelIsCommonFromExt reads SaaS common-model flag from Ext (open-core safe).
func ModelIsCommonFromExt(m *models.ModelType) bool {
	if m == nil || m.Ext == nil {
		return false
	}
	v, ok := m.Ext["is_common_model"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// BuildModelPhysicalHealth diffs expected vs physical columns and adds warnings.
func BuildModelPhysicalHealth(
	model *models.ModelType,
	tableExists bool,
	physicalColumns []string,
) ModelPhysicalHealth {
	name := ""
	if model != nil {
		name = model.Name
	}
	expected := ExpectedPhysicalColumns(model)
	phys := normalizeColumnList(physicalColumns)
	expNorm := normalizeColumnList(expected)

	physSet := toLowerSet(phys)
	expSet := toLowerSet(expNorm)

	var missing, extra []string
	for _, c := range expNorm {
		if _, ok := physSet[strings.ToLower(c)]; !ok {
			missing = append(missing, c)
		}
	}
	for _, c := range phys {
		if _, ok := expSet[strings.ToLower(c)]; !ok {
			extra = append(extra, c)
		}
	}
	sort.Strings(missing)
	sort.Strings(extra)

	h := ModelPhysicalHealth{
		ModelName:       name,
		TableExists:     tableExists,
		PhysicalColumns: phys,
		ExpectedColumns: expNorm,
		MissingColumns:  missing,
		ExtraColumns:    extra,
		IsCommonModel:   ModelIsCommonFromExt(model),
		Warnings:        nil,
	}

	if !tableExists {
		h.Warnings = append(h.Warnings, "physical table missing — publish schema DDL or create the table via Console/Studio ops (MCP cannot apply DDL)")
		return h
	}
	if len(phys) == 1 && strings.EqualFold(phys[0], "id") && len(expNorm) > 1 {
		h.Warnings = append(h.Warnings, "table exists but only id column — likely stub from runModelMigrations; field columns never applied")
	}
	if len(missing) > 0 {
		h.Warnings = append(h.Warnings, "published schema fields missing as physical columns — repair via Console publish/Studio ops (MCP will not run DDL)")
	}
	return h
}

func normalizeColumnList(cols []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, c := range cols {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		key := strings.ToLower(c)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i]) < strings.ToLower(out[j])
	})
	return out
}

func toLowerSet(cols []string) map[string]struct{} {
	m := make(map[string]struct{}, len(cols))
	for _, c := range cols {
		m[strings.ToLower(c)] = struct{}{}
	}
	return m
}

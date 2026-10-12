package schemadraft

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/apito-io/engine/models"
)

// EncodeDiff serializes a schema diff for the GraphQL string field.
func EncodeDiff(d *models.SchemaDiff) string {
	if d == nil {
		d = &models.SchemaDiff{}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func ComputeDiff(active, draft *models.ProjectSchema) *models.SchemaDiff {
	diff := &models.SchemaDiff{}
	if draft == nil {
		return diff
	}
	if active == nil {
		active = &models.ProjectSchema{}
	}

	activeModels := map[string]*models.ModelType{}
	for _, m := range active.Models {
		if m != nil && m.Name != "" {
			activeModels[m.Name] = m
		}
	}
	draftModels := map[string]*models.ModelType{}
	for _, m := range draft.Models {
		if m != nil && m.Name != "" {
			draftModels[m.Name] = m
		}
	}

	for name, dm := range draftModels {
		am, ok := activeModels[name]
		if !ok {
			diff.ModelsAdded++
			diff.ModelChanges = append(diff.ModelChanges, &models.SchemaModelDiff{
				ModelName:           name,
				Added:               true,
				RequiresPhysicalDDL: true,
				FieldsAdded:         append([]*models.FieldInfo{}, dm.Fields...),
				ConnectionsAdded:    append([]*models.ConnectionType{}, dm.Connections...),
			})
			continue
		}
		if md := diffModel(am, dm); md != nil {
			diff.ModelsModified++
			diff.ModelChanges = append(diff.ModelChanges, md)
		}
	}
	for name, am := range activeModels {
		if _, ok := draftModels[name]; !ok {
			diff.ModelsRemoved++
			diff.Destructive = true
			diff.Warnings = append(diff.Warnings, "Model "+name+" will be removed")
			diff.ModelChanges = append(diff.ModelChanges, &models.SchemaModelDiff{
				ModelName:           name,
				Removed:             true,
				RequiresPhysicalDDL: true,
				FieldsRemoved:       append([]*models.FieldInfo{}, am.Fields...),
				ConnectionsRemoved:  append([]*models.ConnectionType{}, am.Connections...),
			})
		}
	}
	annotateConnectionModifications(diff, active, draft)
	SummarizeDiffImpact(diff)
	return diff
}

type ForwardEdge struct {
	FromModel string
	ToModel   string
	KnownAs   string
	FromRel   string
	ToRel     string
}

func ForwardEdges(schema *models.ProjectSchema) map[string]ForwardEdge {
	out := map[string]ForwardEdge{}
	if schema == nil {
		return out
	}
	for _, model := range schema.Models {
		if model == nil {
			continue
		}
		for _, conn := range model.Connections {
			if conn == nil || strings.TrimSpace(conn.Type) != "forward" {
				continue
			}
			peer := FindModel(schema, conn.Model)
			ToRel := ""
			if peer != nil {
				if rev := FindConnection(peer.Connections, model.Name, conn.KnownAs); rev != nil {
					ToRel = rev.Relation
				}
			}
			key := model.Name + "|" + conn.Model + "|" + conn.KnownAs
			out[key] = ForwardEdge{FromModel: model.Name, ToModel: conn.Model, KnownAs: conn.KnownAs, FromRel: conn.Relation, ToRel: ToRel}
		}
	}
	return out
}

func annotateConnectionModifications(diff *models.SchemaDiff, active, draft *models.ProjectSchema) {
	if diff == nil {
		return
	}
	before := ForwardEdges(active)
	after := ForwardEdges(draft)
	for key, next := range after {
		prev, ok := before[key]
		if !ok {
			continue
		}
		if prev.FromRel == next.FromRel && prev.ToRel == next.ToRel {
			continue
		}
		mc := modelChangeFor(diff, next.FromModel)
		mc.ConnectionsModified = append(mc.ConnectionsModified, &models.SchemaConnectionChange{
			Model: next.ToModel, KnownAs: next.KnownAs,
			BeforeRelation: prev.FromRel, AfterRelation: next.FromRel,
			PeerBeforeRelation: prev.ToRel, PeerAfterRelation: next.ToRel,
			Impact: models.SchemaImpactPhysicalDDL,
		})
	}
}

func modelChangeFor(diff *models.SchemaDiff, name string) *models.SchemaModelDiff {
	for _, mc := range diff.ModelChanges {
		if mc != nil && mc.ModelName == name && !mc.Added && !mc.Removed {
			return mc
		}
	}
	mc := &models.SchemaModelDiff{ModelName: name}
	diff.ModelChanges = append(diff.ModelChanges, mc)
	diff.ModelsModified++
	return mc
}

func ConnectionIdentity(c *models.ConnectionType) string {
	if c == nil {
		return ""
	}
	return c.Model + "|" + c.Type + "|" + c.KnownAs
}

func connectionIdentityMap(conns []*models.ConnectionType) map[string]*models.ConnectionType {
	out := make(map[string]*models.ConnectionType, len(conns))
	for _, c := range conns {
		if c == nil {
			continue
		}
		out[ConnectionIdentity(c)] = c
	}
	return out
}

func diffModel(active, draft *models.ModelType) *models.SchemaModelDiff {
	if active == nil || draft == nil {
		return nil
	}
	out := &models.SchemaModelDiff{ModelName: draft.Name}
	diffFieldLevel(out, "", active.Fields, draft.Fields)
	activeConn := connectionIdentityMap(active.Connections)
	draftConn := connectionIdentityMap(draft.Connections)
	for k, c := range draftConn {
		if _, ok := activeConn[k]; !ok {
			out.ConnectionsAdded = append(out.ConnectionsAdded, c)
		}
	}
	for k, c := range activeConn {
		if _, ok := draftConn[k]; !ok {
			out.ConnectionsRemoved = append(out.ConnectionsRemoved, c)
		}
	}
	diffUniqueKeys(out, active.UniqueKeys, draft.UniqueKeys)
	if len(out.FieldsAdded)+len(out.FieldsRemoved)+len(out.FieldsModified)+
		len(out.ConnectionsAdded)+len(out.ConnectionsRemoved)+len(out.ConnectionsModified)+
		len(out.UniqueKeysAdded)+len(out.UniqueKeysRemoved)+len(out.UniqueKeysModified) == 0 {
		return nil
	}
	return out
}

// diffFieldLevel compares fields at one nesting level (parentID == "" for root) and
// recurses into SubFieldInfo so nested repeated/object subfield changes are reported
// against the actual child identifier + parent, not the parent container.
func diffFieldLevel(out *models.SchemaModelDiff, parentID string, activeFields, draftFields []*models.FieldInfo) {
	activeMap := FieldMap(activeFields)
	draftMap := FieldMap(draftFields)
	for id, f := range draftMap {
		if _, ok := activeMap[id]; !ok {
			out.FieldsAdded = append(out.FieldsAdded, f)
		}
	}
	for id, f := range activeMap {
		if _, ok := draftMap[id]; !ok {
			out.FieldsRemoved = append(out.FieldsRemoved, f)
		}
	}
	for id, after := range draftMap {
		before, ok := activeMap[id]
		if !ok {
			continue
		}
		if impact, changes := ClassifyFieldChange(before, after); impact != models.SchemaImpactNoOp {
			out.FieldsModified = append(out.FieldsModified, &models.SchemaFieldChange{
				Identifier:  id,
				ParentField: parentID,
				Label:       after.Label,
				Impact:      impact,
				Changes:     changes,
			})
		}
		if len(before.SubFieldInfo) > 0 || len(after.SubFieldInfo) > 0 {
			diffFieldLevel(out, id, before.SubFieldInfo, after.SubFieldInfo)
		}
	}
}

func FieldMap(fields []*models.FieldInfo) map[string]*models.FieldInfo {
	m := make(map[string]*models.FieldInfo)
	for _, f := range fields {
		if f == nil || f.Identifier == "" {
			continue
		}
		m[f.Identifier] = f
	}
	return m
}

func ConnKeySet(conns []*models.ConnectionType) map[string]*models.ConnectionType {
	m := make(map[string]*models.ConnectionType)
	for _, c := range conns {
		if c == nil {
			continue
		}
		k := c.Model + "|" + c.Relation + "|" + c.Type + "|" + c.KnownAs
		m[k] = c
	}
	return m
}

func diffUniqueKeys(out *models.SchemaModelDiff, before, after []*models.UniqueKey) {
	if out == nil {
		return
	}
	beforeMap := uniqueKeyMap(before)
	afterMap := uniqueKeyMap(after)
	for name, key := range afterMap {
		prev, ok := beforeMap[name]
		if !ok {
			out.UniqueKeysAdded = append(out.UniqueKeysAdded, key)
			continue
		}
		if strings.Join(prev.Columns, "\x00") != strings.Join(key.Columns, "\x00") {
			out.UniqueKeysModified = append(out.UniqueKeysModified, &models.SchemaUniqueKeyChange{
				Name:    name,
				Columns: key.Columns,
				Impact:  models.SchemaImpactPhysicalDDL,
				Changes: []string{"columns: " + strings.Join(prev.Columns, ", ") + " → " + strings.Join(key.Columns, ", ")},
			})
		}
	}
	for name, key := range beforeMap {
		if _, ok := afterMap[name]; !ok {
			out.UniqueKeysRemoved = append(out.UniqueKeysRemoved, key)
		}
	}
}

func uniqueKeyMap(keys []*models.UniqueKey) map[string]*models.UniqueKey {
	m := make(map[string]*models.UniqueKey, len(keys))
	for _, k := range keys {
		if k == nil || strings.TrimSpace(k.Name) == "" {
			continue
		}
		m[strings.ToLower(strings.TrimSpace(k.Name))] = k
	}
	return m
}

func summarizeDiff(d *models.SchemaDiff) {
	if d == nil {
		return
	}
	d.FieldsAdded = 0
	d.FieldsRemoved = 0
	d.ConnectionsAdded = 0
	d.ConnectionsRemoved = 0
	for _, mc := range d.ModelChanges {
		d.FieldsAdded += len(mc.FieldsAdded)
		d.FieldsRemoved += len(mc.FieldsRemoved)
		d.ConnectionsAdded += len(mc.ConnectionsAdded)
		d.ConnectionsRemoved += len(mc.ConnectionsRemoved)
		if mc.Removed {
			d.Destructive = true
		}
	}
}

// ClassifyFieldChange compares two fields with the same identifier.
func ClassifyFieldChange(before, after *models.FieldInfo) (impact string, changes []string) {
	if before == nil || after == nil {
		return models.SchemaImpactMetadataOnly, nil
	}
	if before.FieldType != after.FieldType || before.InputType != after.InputType {
		changes = append(changes, "field_type: "+before.FieldType+" → "+after.FieldType)
		return models.SchemaImpactPhysicalDDL, changes
	}
	if before.Identifier != after.Identifier {
		changes = append(changes, "identifier: "+before.Identifier+" → "+after.Identifier)
		return models.SchemaImpactPhysicalDDL, changes
	}
	if !validationEqual(before.Validation, after.Validation) {
		changes = append(changes, "validation changed")
		// Unique/index constraints in validation may imply physical DDL; default metadata unless type changed.
		if validationImpliesPhysicalDDL(before.Validation, after.Validation) {
			return models.SchemaImpactPhysicalDDL, changes
		}
		return models.SchemaImpactMetadataOnly, changes
	}
	if before.Label != after.Label ||
		before.Description != after.Description ||
		before.FieldSubType != after.FieldSubType ||
		before.ParentField != after.ParentField ||
		before.Serial != after.Serial ||
		before.EnableIndexing != after.EnableIndexing ||
		before.SystemGenerated != after.SystemGenerated ||
		before.IsObjectField != after.IsObjectField {
		if before.Label != after.Label {
			changes = append(changes, "label: "+before.Label+" → "+after.Label)
		}
		if before.FieldSubType != after.FieldSubType {
			changes = append(changes, "field_sub_type: "+before.FieldSubType+" → "+after.FieldSubType)
		}
		if before.Description != after.Description {
			changes = append(changes, "description changed")
		}
		if before.Serial != after.Serial {
			changes = append(changes, fmt.Sprintf("position: %d → %d", before.Serial, after.Serial))
		}
		return models.SchemaImpactMetadataOnly, changes
	}
	// Nested subfield changes are handled by diffFieldLevel recursion, which reports the
	// actual changed child against its parent identifier; the parent container itself is NoOp.
	return models.SchemaImpactNoOp, nil
}

func validationImpliesPhysicalDDL(before, after *models.Validation) bool {
	if before == nil && after == nil {
		return false
	}
	// Fixed list / choice metadata is not physical DDL.
	if before != nil && after != nil {
		if !reflect.DeepEqual(before.FixedListElements, after.FixedListElements) {
			return false
		}
	}
	return false
}

func validationEqual(a, b *models.Validation) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}

func DiffRequiresPhysicalDDL(d *models.SchemaDiff) bool {
	if d == nil {
		return false
	}
	if d.RequiresPhysicalDDL {
		return true
	}
	for _, mc := range d.ModelChanges {
		if mc == nil {
			continue
		}
		if mc.Added || mc.Removed || mc.RequiresPhysicalDDL {
			return true
		}
		if len(mc.FieldsAdded)+len(mc.FieldsRemoved)+len(mc.ConnectionsAdded)+len(mc.ConnectionsRemoved)+len(mc.ConnectionsModified) > 0 {
			return true
		}
		if len(mc.UniqueKeysAdded)+len(mc.UniqueKeysRemoved)+len(mc.UniqueKeysModified) > 0 {
			return true
		}
		for _, fc := range mc.FieldsModified {
			if fc != nil && fc.Impact == models.SchemaImpactPhysicalDDL {
				return true
			}
		}
	}
	return false
}

func SummarizeDiffImpact(d *models.SchemaDiff) {
	if d == nil {
		return
	}
	summarizeDiff(d)
	d.FieldsModified = 0
	d.RequiresPhysicalDDL = false
	for _, mc := range d.ModelChanges {
		if mc == nil {
			continue
		}
		d.FieldsModified += len(mc.FieldsModified)
		if mc.Added || mc.Removed {
			mc.RequiresPhysicalDDL = true
			d.RequiresPhysicalDDL = true
		}
		if len(mc.FieldsAdded)+len(mc.FieldsRemoved)+len(mc.ConnectionsAdded)+len(mc.ConnectionsRemoved)+len(mc.ConnectionsModified) > 0 {
			mc.RequiresPhysicalDDL = true
			d.RequiresPhysicalDDL = true
		}
		if len(mc.UniqueKeysAdded)+len(mc.UniqueKeysRemoved)+len(mc.UniqueKeysModified) > 0 {
			mc.RequiresPhysicalDDL = true
			d.RequiresPhysicalDDL = true
		}
		for _, fc := range mc.FieldsModified {
			if fc != nil && fc.Impact == models.SchemaImpactPhysicalDDL {
				mc.RequiresPhysicalDDL = true
				d.RequiresPhysicalDDL = true
			}
		}
	}
}

func FieldChangeSummary(fc *models.SchemaFieldChange) string {
	if fc == nil {
		return "field changed"
	}
	if len(fc.Changes) > 0 {
		return "field " + fc.Identifier + ": " + strings.Join(fc.Changes, "; ")
	}
	return "field " + fc.Identifier + " updated"
}

package schemadraft

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// Operation is one staged schema mutation. The public draft file and the Cloud changeset replay this same shape.
type Operation struct {
	ID          string
	Type        string
	ModelName   string
	FieldName   string
	Payload     map[string]interface{}
	Summary     string
	Destructive bool
	CreatedAt   string
}

func ReplayOperations(base *models.ProjectSchema, ops []*Operation) (*models.ProjectSchema, error) {
	draft, err := models.CloneProjectSchema(base)
	if err != nil {
		return nil, err
	}
	// Published snapshots omit an unconnected runtime auth-users descriptor.
	// Relation operations are nevertheless allowed to target this reserved
	// model, so replay must inject it before validating/applying those ops.
	// Once connected, StripProjectAuthUserModelFromPersistedSchema preserves
	// the descriptor because it contains the inverse relation metadata.
	models.EnsureProjectAuthUserModelInSchema(draft)
	for _, op := range ops {
		if op == nil {
			continue
		}
		if err := Apply(draft, op); err != nil {
			return nil, err
		}
	}
	models.DedupeProjectSchemaFields(draft)
	return draft, nil
}

func Apply(draft *models.ProjectSchema, op *Operation) error {
	if draft == nil || op == nil {
		return fmt.Errorf("invalid draft operation")
	}
	switch op.Type {
	case "add_model":
		return draftAddModel(draft, op)
	case "update_model":
		return draftUpdateModel(draft, op)
	case "delete_model":
		return draftDeleteModel(draft, op)
	case "rename_model":
		return draftRenameModel(draft, op)
	case "duplicate_model":
		return draftDuplicateModel(draft, op)
	case "convert_model":
		return draftConvertModel(draft, op)
	case "add_field":
		return draftAddField(draft, op)
	case "create_connection":
		return DraftCreateConnection(draft, op)
	case "update_connection":
		return draftUpdateConnection(draft, op)
	case "revert_schema_version":
		return draftRevertSchema(draft, op)
	case "delete_connection":
		return draftDeleteConnection(draft, op)
	case "upsert_unique_key":
		return draftUpsertUniqueKey(draft, op)
	case "delete_unique_key":
		return draftDeleteUniqueKey(draft, op)
	case "model_field_operation":
		return draftModelFieldOp(draft, op)
	case "rearrange_field":
		return draftRearrangeField(draft, op)
	default:
		return fmt.Errorf("unsupported draft operation type %q", op.Type)
	}
}

func draftAddModel(draft *models.ProjectSchema, op *Operation) error {
	name, _ := op.Payload["name"].(string)
	if name == "" {
		name = op.ModelName
	}
	name, err := utility.LegacyStoredNameToCanonical(name)
	if err != nil {
		return err
	}
	if models.ModelNameIsReservedProjectAuthUser(name) {
		return fmt.Errorf("model %q is reserved for application auth users (runtime-injected; use add_relation to link models to users)", name)
	}
	if FindModel(draft, name) != nil {
		return fmt.Errorf("model %q already exists in draft", name)
	}
	single, _ := op.Payload["single_record"].(bool)
	model := &models.ModelType{Name: name, SinglePage: single}
	if v, ok := op.Payload["is_common_model"].(bool); ok {
		models.PatchModelClassification(model, nil, &v)
	}
	draft.Models = append(draft.Models, model)
	return nil
}

func draftUpdateModel(draft *models.ProjectSchema, op *Operation) error {
	name := StringArg(op.Payload, "model_name")
	if name == "" {
		name = op.ModelName
	}
	m := FindModel(draft, name)
	if m == nil {
		return fmt.Errorf("model %q not found in draft", name)
	}
	common, hasCommon := BoolPayload(op.Payload, "is_common_model")
	if !hasCommon {
		return nil
	}
	models.PatchModelClassification(m, nil, &common)
	return nil
}

func BoolPayload(args map[string]interface{}, key string) (bool, bool) {
	if args == nil {
		return false, false
	}
	v, ok := args[key].(bool)
	return v, ok
}

func draftDeleteModel(draft *models.ProjectSchema, op *Operation) error {
	name := op.ModelName
	if name == "" {
		name, _ = op.Payload["model_name"].(string)
	}
	draft.Models = removeModelByName(draft.Models, name)
	return nil
}

func draftRenameModel(draft *models.ProjectSchema, op *Operation) error {
	oldName := StringArg(op.Payload, "model_name")
	newName := StringArg(op.Payload, "new_name")
	m := FindModel(draft, oldName)
	if m == nil {
		return fmt.Errorf("model %q not found in draft", oldName)
	}
	newName, err := utility.LegacyStoredNameToCanonical(newName)
	if err != nil {
		return err
	}
	if FindModel(draft, newName) != nil && !strings.EqualFold(oldName, newName) {
		return fmt.Errorf("model %q already exists", newName)
	}
	m.Name = newName
	rewriteConnectionModelRefs(draft, oldName, newName)
	return nil
}

// rewriteConnectionModelRefs updates every connection that points at oldName so
// peer models do not show false remove/add connection diffs after a rename.
func rewriteConnectionModelRefs(draft *models.ProjectSchema, oldName, newName string) {
	if draft == nil || oldName == "" || newName == "" || strings.EqualFold(oldName, newName) {
		return
	}
	for _, model := range draft.Models {
		if model == nil {
			continue
		}
		for _, c := range model.Connections {
			if c == nil {
				continue
			}
			if strings.EqualFold(c.Model, oldName) {
				c.Model = newName
			}
			if c.KnownAs != "" && strings.EqualFold(c.KnownAs, oldName) {
				c.KnownAs = newName
			}
		}
	}
}

func draftDuplicateModel(draft *models.ProjectSchema, op *Operation) error {
	src := StringArg(op.Payload, "model_name")
	newName := StringArg(op.Payload, "new_name")
	srcM := FindModel(draft, src)
	if srcM == nil {
		return fmt.Errorf("model %q not found", src)
	}
	newName, err := utility.LegacyStoredNameToCanonical(newName)
	if err != nil {
		return err
	}
	if FindModel(draft, newName) != nil {
		return fmt.Errorf("model %q already exists", newName)
	}
	clone, err := cloneModelType(srcM)
	if err != nil {
		return err
	}
	clone.Name = newName
	draft.Models = append(draft.Models, clone)
	return nil
}

func draftConvertModel(draft *models.ProjectSchema, op *Operation) error {
	name := StringArg(op.Payload, "model_name")
	m := FindModel(draft, name)
	if m == nil {
		return fmt.Errorf("model %q not found", name)
	}
	m.SinglePage = !m.SinglePage
	return nil
}

func draftAddField(draft *models.ProjectSchema, op *Operation) error {
	modelName := StringArg(op.Payload, "model_name")
	if modelName == "" {
		modelName = op.ModelName
	}
	var err error
	modelName, err = utility.LegacyStoredNameToCanonical(modelName)
	if err != nil {
		return err
	}
	m := FindModel(draft, modelName)
	if m == nil {
		return fmt.Errorf("model %q not found in draft", modelName)
	}
	label := StringArg(op.Payload, "field_label")
	fieldType := StringArg(op.Payload, "field_type")
	identifier := LabelToIdentifier(label, fieldType)
	if identifier == "" {
		identifier = StringArg(op.Payload, "field")
	}
	if identifier == "" {
		return fmt.Errorf("field label is required")
	}
	isUpdate, _ := op.Payload["is_update"].(bool)
	parentField := StringArg(op.Payload, "parent_field")
	targetFields, err := draftFieldSliceForParent(m, parentField)
	if err != nil {
		if parentField != "" && parentField != "_root" {
			return fmt.Errorf(
				"parent field %q not found in draft (model %q, child %q, op %q) — stage the parent object/repeated field before nested children",
				parentField, modelName, identifier, op.ID,
			)
		}
		return err
	}
	if !isUpdate && FindFieldByIdentifier(*targetFields, identifier) != nil {
		return fmt.Errorf("a field with identifier %q already exists in draft", identifier)
	}
	f := &models.FieldInfo{
		Identifier: identifier,
		Label:      label,
		FieldType:  fieldType,
	}
	if sub, ok := op.Payload["field_sub_type"].(string); ok {
		f.FieldSubType = sub
	}
	if in, ok := op.Payload["input_type"].(string); ok {
		f.InputType = in
	}
	if desc, ok := op.Payload["field_description"].(string); ok {
		f.Description = desc
	}
	f.ParentField = parentField
	if v, ok := op.Payload["validation"].(map[string]interface{}); ok {
		f.Validation = decodeValidation(v)
	}
	if serial, ok := op.Payload["serial"].(int); ok {
		f.Serial = uint32(serial)
	}
	if f.Serial == 0 {
		f.Serial = uint32(len(*targetFields) + 1)
	}
	if isUpdate {
		if existing := FindFieldByIdentifier(*targetFields, identifier); existing != nil {
			mergeFieldPayload(existing, op.Payload)
			existing.ParentField = parentField
		} else {
			*targetFields = upsertFieldInModel(*targetFields, f)
		}
	} else {
		*targetFields = upsertFieldInModel(*targetFields, f)
	}
	return nil
}

var ErrConnectionAlreadyInDraft = fmt.Errorf("connection already exists in draft")

// validateCreateConnectionAgainstDraft enforces draft-side prerequisites for staging
// create_connection (versioning path skips open-core live-schema defaultFn).
func ValidateCreateConnectionAgainstDraft(draft *models.ProjectSchema, op *Operation) error {
	if op == nil {
		return fmt.Errorf("invalid create_connection operation")
	}
	from := StringArg(op.Payload, "from")
	to := StringArg(op.Payload, "to")
	knownAs := StringArg(op.Payload, "known_as")
	fwdType := StringArg(op.Payload, "forward_connection_type")
	revType := StringArg(op.Payload, "reverse_connection_type")
	if from == "" || to == "" {
		return fmt.Errorf("create_connection requires from and to model names")
	}
	fm := FindModel(draft, from)
	tm := FindModel(draft, to)
	if fm == nil || tm == nil {
		missing := make([]string, 0, 2)
		if fm == nil {
			missing = append(missing, from)
		}
		if tm == nil {
			missing = append(missing, to)
		}
		return fmt.Errorf(
			"cannot stage connection %q → %q: model(s) %v not in draft — create_model both sides before add_relation",
			from, to, missing,
		)
	}
	fromConn := FindConnection(fm.Connections, to, knownAs)
	toConn := FindConnection(tm.Connections, from, knownAs)
	if fromConn != nil && toConn != nil &&
		!connectionNeedsNormalize(fromConn, "forward", fwdType) &&
		!connectionNeedsNormalize(toConn, "backward", revType) {
		return ErrConnectionAlreadyInDraft
	}
	return nil
}

func connectionExists(conns []*models.ConnectionType, peer, knownAs string) bool {
	return FindConnection(conns, peer, knownAs) != nil
}

func FindConnection(conns []*models.ConnectionType, peer, knownAs string) *models.ConnectionType {
	for _, c := range conns {
		if c != nil && c.Model == peer && c.KnownAs == knownAs {
			return c
		}
	}
	return nil
}

// connectionNeedsNormalize is true when Type or Relation do not match the desired
// forward/backward ownership and cardinality from upsertConnectionToModel.
func connectionNeedsNormalize(c *models.ConnectionType, wantType, wantRelation string) bool {
	if c == nil {
		return true
	}
	gotType := strings.ToLower(strings.TrimSpace(c.Type))
	want := strings.ToLower(strings.TrimSpace(wantType))
	if gotType != want {
		return true
	}
	if wantRelation == "" {
		return false
	}
	return strings.ToLower(strings.TrimSpace(c.Relation)) != strings.ToLower(strings.TrimSpace(wantRelation))
}

func DraftCreateConnection(draft *models.ProjectSchema, op *Operation) error {
	from := StringArg(op.Payload, "from")
	to := StringArg(op.Payload, "to")
	knownAs := StringArg(op.Payload, "known_as")
	fwdType := StringArg(op.Payload, "forward_connection_type")
	revType := StringArg(op.Payload, "reverse_connection_type")
	fm := FindModel(draft, from)
	tm := FindModel(draft, to)
	if fm == nil || tm == nil {
		return fmt.Errorf("connection models not found in draft (from=%q to=%q)", from, to)
	}
	fromConn := FindConnection(fm.Connections, to, knownAs)
	toConn := FindConnection(tm.Connections, from, knownAs)
	if fromConn != nil && toConn != nil && connectionStorageChanges(fromConn.Relation, toConn.Relation, fwdType, revType) {
		return fmt.Errorf("cardinality change on %s -> %s must use update_connection", from, to)
	}
	if fromConn != nil {
		fromConn.Type = "forward"
		if fwdType != "" {
			fromConn.Relation = fwdType
		}
	} else {
		fm.Connections = append(fm.Connections, &models.ConnectionType{
			Model: to, Relation: fwdType, Type: "forward", KnownAs: knownAs,
		})
	}
	if toConn != nil {
		toConn.Type = "backward"
		if revType != "" {
			toConn.Relation = revType
		}
	} else {
		tm.Connections = append(tm.Connections, &models.ConnectionType{
			Model: from, Relation: revType, Type: "backward", KnownAs: knownAs,
		})
	}
	return nil
}

func draftUpdateConnection(draft *models.ProjectSchema, op *Operation) error {
	from := StringArg(op.Payload, "from")
	to := StringArg(op.Payload, "to")
	knownAs := StringArg(op.Payload, "known_as")
	fwdType := StringArg(op.Payload, "target_from_relation")
	if fwdType == "" {
		fwdType = StringArg(op.Payload, "forward_connection_type")
	}
	revType := StringArg(op.Payload, "target_to_relation")
	if revType == "" {
		revType = StringArg(op.Payload, "reverse_connection_type")
	}
	fm := FindModel(draft, from)
	tm := FindModel(draft, to)
	if fm == nil || tm == nil {
		return fmt.Errorf("connection models not found in draft (from=%q to=%q)", from, to)
	}
	fromConn := FindConnection(fm.Connections, to, knownAs)
	toConn := FindConnection(tm.Connections, from, knownAs)
	if fromConn == nil || toConn == nil {
		return fmt.Errorf("update_connection requires an existing edge %s -> %s", from, to)
	}
	fromConn.Type = "forward"
	fromConn.Relation = fwdType
	toConn.Type = "backward"
	toConn.Relation = revType
	syncSyntheticRelationField(fm, to, knownAs, fwdType)
	syncSyntheticRelationField(tm, from, knownAs, revType)
	return nil
}

func draftRevertSchema(draft *models.ProjectSchema, op *Operation) error {
	if draft == nil || op == nil {
		return fmt.Errorf("invalid revert")
	}
	raw := StringArg(op.Payload, "target_schema_json")
	if raw == "" {
		return fmt.Errorf("revert_schema_version is missing target_schema_json")
	}
	var target models.ProjectSchema
	if err := json.Unmarshal([]byte(raw), &target); err != nil {
		return err
	}
	*draft = target
	return nil
}

func connectionStorageChanges(fromRel, toRel, targetFrom, targetTo string) bool {
	return relationShape(fromRel, toRel) != relationShape(targetFrom, targetTo)
}

func relationShape(fromRel, toRel string) string {
	fromRel = strings.ToLower(strings.TrimSpace(fromRel))
	toRel = strings.ToLower(strings.TrimSpace(toRel))
	switch {
	case fromRel == "has_one" && toRel == "has_one":
		return "1:1"
	case fromRel == "has_many" && toRel == "has_one":
		return "1:N"
	case fromRel == "has_one" && toRel == "has_many":
		return "N:1"
	case fromRel == "has_many" && toRel == "has_many":
		return "N:N"
	default:
		return ""
	}
}

func syncSyntheticRelationField(model *models.ModelType, peer, knownAs, relation string) {
	if model == nil {
		return
	}
	identifier := utility.SyntheticSystemRelationFieldIdentifier(peer, knownAs)
	if identifier == "" {
		return
	}
	if strings.EqualFold(relation, "has_one") {
		for _, f := range model.Fields {
			if f != nil && f.Identifier == identifier {
				return
			}
		}
		model.Fields = append(model.Fields, &models.FieldInfo{
			Identifier: identifier, Description: "An Auto Generated Relation Identifier for Easy Filter Purposes",
			InputType: "string", FieldType: "text", SystemGenerated: true,
			Validation: &models.Validation{Hide: true}, Label: "System " + peer + " ID",
		})
		return
	}
	out := model.Fields[:0]
	for _, f := range model.Fields {
		if f != nil && f.Identifier == identifier && f.SystemGenerated {
			continue
		}
		out = append(out, f)
	}
	model.Fields = out
}

func draftDeleteConnection(draft *models.ProjectSchema, op *Operation) error {
	from := StringArg(op.Payload, "from")
	to := StringArg(op.Payload, "to")
	knownAs := StringArg(op.Payload, "known_as")
	fm := FindModel(draft, from)
	tm := FindModel(draft, to)
	if fm == nil || tm == nil {
		return fmt.Errorf("connection models not found")
	}
	fm.Connections = removeConnection(fm.Connections, to, knownAs)
	tm.Connections = removeConnection(tm.Connections, from, knownAs)
	return nil
}

func draftUpsertUniqueKey(draft *models.ProjectSchema, op *Operation) error {
	modelName := StringArg(op.Payload, "model_name")
	if modelName == "" {
		modelName = op.ModelName
	}
	m := FindModel(draft, modelName)
	if m == nil {
		return fmt.Errorf("model %q not found in draft", modelName)
	}
	key, err := utility.ValidateUniqueKey(m, StringArg(op.Payload, "name"), StringList(op.Payload, "columns"))
	if err != nil {
		return err
	}
	replaced := false
	for i, existing := range m.UniqueKeys {
		if existing != nil && strings.EqualFold(existing.Name, key.Name) {
			m.UniqueKeys[i] = key
			replaced = true
			break
		}
	}
	if !replaced {
		m.UniqueKeys = append(m.UniqueKeys, key)
	}
	return nil
}

func draftDeleteUniqueKey(draft *models.ProjectSchema, op *Operation) error {
	modelName := StringArg(op.Payload, "model_name")
	if modelName == "" {
		modelName = op.ModelName
	}
	m := FindModel(draft, modelName)
	if m == nil {
		return fmt.Errorf("model %q not found in draft", modelName)
	}
	name := strings.ToLower(strings.TrimSpace(StringArg(op.Payload, "name")))
	if name == "" {
		return fmt.Errorf("unique key name is required")
	}
	out := make([]*models.UniqueKey, 0, len(m.UniqueKeys))
	found := false
	for _, k := range m.UniqueKeys {
		if k != nil && strings.EqualFold(k.Name, name) {
			found = true
			continue
		}
		out = append(out, k)
	}
	if !found {
		return fmt.Errorf("unique key %q not found on %s", name, modelName)
	}
	m.UniqueKeys = out
	return nil
}

func StringList(args map[string]interface{}, key string) []string {
	if args == nil {
		return nil
	}
	switch v := args[key].(type) {
	case []string:
		return append([]string(nil), v...)
	case []interface{}:
		out := make([]string, 0, len(v))
		for _, item := range v {
			s, ok := item.(string)
			if !ok || strings.TrimSpace(s) == "" {
				continue
			}
			out = append(out, s)
		}
		return out
	default:
		return nil
	}
}

func draftModelFieldOp(draft *models.ProjectSchema, op *Operation) error {
	modelName := StringArg(op.Payload, "model_name")
	fieldName := StringArg(op.Payload, "field_name")
	parentField := StringArg(op.Payload, "parent_field")
	opType := strings.ToLower(StringArg(op.Payload, "type"))
	m := FindModel(draft, modelName)
	if m == nil {
		return fmt.Errorf("model %q not found", modelName)
	}
	targetFields, err := draftFieldSliceForParent(m, parentField)
	if err != nil {
		if parentField != "" && parentField != "_root" {
			return fmt.Errorf(
				"parent field %q not found in draft (model %q, field %q, op %q) — stage the parent object/repeated field before nested children",
				parentField, modelName, fieldName, op.ID,
			)
		}
		return err
	}
	switch opType {
	case "delete":
		*targetFields = removeFieldByIdentifier(*targetFields, fieldName)
	case "rename":
		newName := StringArg(op.Payload, "new_name")
		for _, f := range *targetFields {
			if f == nil || f.Identifier != fieldName {
				continue
			}
			f.Identifier = newName
			if f.Label == fieldName {
				f.Label = newName
			}
		}
	case "change_type", "changetype":
		changed := StringArg(op.Payload, "changed_type")
		if f := FindFieldByIdentifier(*targetFields, fieldName); f != nil {
			f.FieldType = changed
		}
	case "duplicate":
		newName := StringArg(op.Payload, "new_name")
		src := FindFieldByIdentifier(*targetFields, fieldName)
		if src == nil {
			return fmt.Errorf("field %q not found", fieldName)
		}
		clone, err := CloneFieldInfo(src)
		if err != nil {
			return err
		}
		clone.Identifier = newName
		if clone.Label == fieldName {
			clone.Label = newName
		}
		clone.ParentField = parentField
		*targetFields = upsertFieldInModel(*targetFields, clone)
	case "update", "edit":
		f := FindFieldByIdentifier(*targetFields, fieldName)
		if f == nil {
			return fmt.Errorf("field %q not found", fieldName)
		}
		mergeFieldPayload(f, op.Payload)
	}
	return nil
}

func FindFieldByIdentifier(fields []*models.FieldInfo, id string) *models.FieldInfo {
	for _, f := range fields {
		if f != nil && f.Identifier == id {
			return f
		}
	}
	return nil
}

func draftFieldSliceForParent(model *models.ModelType, parentField string) (*[]*models.FieldInfo, error) {
	if model == nil {
		return nil, fmt.Errorf("model is required")
	}
	if parentField == "" || parentField == "_root" {
		return &model.Fields, nil
	}
	parent := FindFieldByIdentifierDeep(model.Fields, parentField)
	if parent == nil {
		return nil, fmt.Errorf("parent field %q not found in draft", parentField)
	}
	return &parent.SubFieldInfo, nil
}

func CloneFieldInfo(f *models.FieldInfo) (*models.FieldInfo, error) {
	if f == nil {
		return nil, fmt.Errorf("nil field")
	}
	b, err := json.Marshal(f)
	if err != nil {
		return nil, err
	}
	var out models.FieldInfo
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func mergeFieldPayload(f *models.FieldInfo, payload map[string]interface{}) {
	if f == nil || payload == nil {
		return
	}
	if v, ok := payload["field_label"].(string); ok && v != "" {
		f.Label = v
	}
	if v, ok := payload["field_type"].(string); ok && v != "" {
		f.FieldType = v
	}
	if v, ok := payload["field_sub_type"].(string); ok {
		f.FieldSubType = v
	}
	if v, ok := payload["input_type"].(string); ok && v != "" {
		f.InputType = v
	}
	if v, ok := payload["description"].(string); ok {
		f.Description = v
	}
	if v, ok := payload["field_description"].(string); ok {
		f.Description = v
	}
	if val, ok := payload["validation"].(map[string]interface{}); ok {
		f.Validation = decodeValidation(val)
	}
	if serial, ok := payload["serial"].(int); ok {
		f.Serial = uint32(serial)
	}
}

func draftRearrangeField(draft *models.ProjectSchema, op *Operation) error {
	modelName := StringArg(op.Payload, "model_name")
	model := FindModel(draft, modelName)
	if model == nil {
		return fmt.Errorf("model %q not found", modelName)
	}
	fieldName := StringArg(op.Payload, "field_name")
	if fieldName == "" {
		fieldName = StringArg(op.Payload, "dragged_field")
	}
	moveType := StringArg(op.Payload, "move_type")
	newPosition := intArgPayload(op.Payload, "new_position")
	parentID := StringArg(op.Payload, "parent_id")

	switch moveType {
	case "reorder":
		targetFields := &model.Fields
		if parentID != "" {
			parent := FindFieldByIdentifierDeep(model.Fields, parentID)
			if parent == nil {
				return fmt.Errorf("parent field %q not found", parentID)
			}
			targetFields = &parent.SubFieldInfo
		}
		currentPosition := indexFieldByIdentifier(*targetFields, fieldName)
		if currentPosition < 0 {
			return fmt.Errorf("field %q not found", fieldName)
		}
		*targetFields = moveDraftFieldInSlice(*targetFields, currentPosition, newPosition)
	case "child_to_parent":
		if parentID == "" {
			return fmt.Errorf("cannot move to root when parent_id is empty")
		}
		sourceParent := FindFieldByIdentifierDeep(model.Fields, parentID)
		if sourceParent == nil {
			return fmt.Errorf("source parent field %q not found", parentID)
		}
		field := removeDraftFieldByIdentifier(&sourceParent.SubFieldInfo, fieldName)
		if field == nil {
			return fmt.Errorf("field %q not found", fieldName)
		}
		field.ParentField = ""
		model.Fields = insertDraftFieldAtPosition(model.Fields, field, newPosition)
	case "parent_to_child":
		if parentID == "" {
			return fmt.Errorf("parent_id is required for parent_to_child move")
		}
		targetParent := FindFieldByIdentifierDeep(model.Fields, parentID)
		if targetParent == nil {
			return fmt.Errorf("target parent field %q not found", parentID)
		}
		field := removeDraftFieldByIdentifier(&model.Fields, fieldName)
		if field == nil {
			field = removeDraftFieldFromNested(model.Fields, fieldName, parentID)
		}
		if field == nil {
			return fmt.Errorf("field %q not found", fieldName)
		}
		field.ParentField = parentID
		targetParent.SubFieldInfo = insertDraftFieldAtPosition(targetParent.SubFieldInfo, field, newPosition)
	default:
		return fmt.Errorf("invalid move type %q", moveType)
	}

	updateDraftFieldSerials(model.Fields, 1)
	return nil
}

func intArgPayload(args map[string]interface{}, key string) int {
	if args == nil {
		return 0
	}
	switch v := args[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		if math.IsNaN(v) {
			return 0
		}
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	default:
		return 0
	}
}

func indexFieldByIdentifier(fields []*models.FieldInfo, id string) int {
	for i, field := range fields {
		if field != nil && field.Identifier == id {
			return i
		}
	}
	return -1
}

func FindFieldByIdentifierDeep(fields []*models.FieldInfo, id string) *models.FieldInfo {
	// Prefer shallowest match so parent_field "sections" resolves to a root
	// repeated group before a nested group with the same identifier
	// (e.g. class.sections vs class.divisions.sections).
	for _, field := range fields {
		if field != nil && field.Identifier == id {
			return field
		}
	}
	for _, field := range fields {
		if field == nil {
			continue
		}
		if found := FindFieldByIdentifierDeep(field.SubFieldInfo, id); found != nil {
			return found
		}
	}
	return nil
}

func moveDraftFieldInSlice(fields []*models.FieldInfo, fromPos, toPos int) []*models.FieldInfo {
	if fromPos < 0 || fromPos >= len(fields) {
		return fields
	}
	if toPos < 0 {
		toPos = 0
	}
	if toPos >= len(fields) {
		toPos = len(fields) - 1
	}
	field := fields[fromPos]
	fields = append(fields[:fromPos], fields[fromPos+1:]...)
	fields = append(fields, nil)
	copy(fields[toPos+1:], fields[toPos:])
	fields[toPos] = field
	return fields
}

func insertDraftFieldAtPosition(fields []*models.FieldInfo, field *models.FieldInfo, position int) []*models.FieldInfo {
	if position < 0 {
		position = 0
	}
	if position > len(fields) {
		position = len(fields)
	}
	fields = append(fields, nil)
	copy(fields[position+1:], fields[position:])
	fields[position] = field
	return fields
}

func removeDraftFieldByIdentifier(fields *[]*models.FieldInfo, id string) *models.FieldInfo {
	if fields == nil {
		return nil
	}
	for i, field := range *fields {
		if field != nil && field.Identifier == id {
			*fields = append((*fields)[:i], (*fields)[i+1:]...)
			return field
		}
	}
	return nil
}

func removeDraftFieldFromNested(fields []*models.FieldInfo, id, skipParent string) *models.FieldInfo {
	for _, field := range fields {
		if field == nil || field.Identifier == skipParent {
			continue
		}
		if removed := removeDraftFieldByIdentifier(&field.SubFieldInfo, id); removed != nil {
			return removed
		}
		if removed := removeDraftFieldFromNested(field.SubFieldInfo, id, skipParent); removed != nil {
			return removed
		}
	}
	return nil
}

func updateDraftFieldSerials(fields []*models.FieldInfo, startSerial uint32) {
	for i, field := range fields {
		if field == nil {
			continue
		}
		field.Serial = startSerial + uint32(i)
		if len(field.SubFieldInfo) > 0 {
			updateDraftFieldSerials(field.SubFieldInfo, 1)
		}
	}
}

func StringArg(args map[string]interface{}, key string) string {
	if args == nil {
		return ""
	}
	v, ok := args[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	// GraphQL enum args (e.g. enums.FieldOperation) are typed strings, not plain string.
	return fmt.Sprintf("%v", v)
}

func decodeValidation(v map[string]interface{}) *models.Validation {
	if v == nil {
		return nil
	}
	out := &models.Validation{}
	b, _ := json.Marshal(v)
	_ = json.Unmarshal(b, out)
	return out
}

func cloneModelType(m *models.ModelType) (*models.ModelType, error) {
	if m == nil {
		return nil, fmt.Errorf("nil model")
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	var out models.ModelType
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func removeConnection(conns []*models.ConnectionType, peer, knownAs string) []*models.ConnectionType {
	out := make([]*models.ConnectionType, 0, len(conns))
	for _, c := range conns {
		if c != nil && c.Model == peer && c.KnownAs == knownAs {
			continue
		}
		out = append(out, c)
	}
	return out
}

func FindModel(schema *models.ProjectSchema, name string) *models.ModelType {
	if schema == nil {
		return nil
	}
	for _, m := range schema.Models {
		if m != nil && m.Name == name {
			return m
		}
	}
	return nil
}

func removeModelByName(list []*models.ModelType, name string) []*models.ModelType {
	out := make([]*models.ModelType, 0, len(list))
	for _, m := range list {
		if m != nil && m.Name != name {
			out = append(out, m)
		}
	}
	return out
}

func upsertFieldInModel(fields []*models.FieldInfo, f *models.FieldInfo) []*models.FieldInfo {
	for i, existing := range fields {
		if existing != nil && existing.Identifier == f.Identifier {
			fields[i] = f
			return fields
		}
	}
	return append(fields, f)
}

func removeFieldByIdentifier(fields []*models.FieldInfo, id string) []*models.FieldInfo {
	out := make([]*models.FieldInfo, 0, len(fields))
	for _, f := range fields {
		if f != nil && f.Identifier != id {
			out = append(out, f)
		}
	}
	return out
}

func LabelToIdentifier(label, fieldType string) string {
	if label == "" {
		return ""
	}
	valid, err := utility.IsValidIdentifier(label)
	if err != nil {
		return ""
	}
	return valid.Identifier
}

func OperationSummary(opType string, args map[string]interface{}) string {
	switch opType {
	case models.SchemaOpTypeCreateModel:
		return fmt.Sprintf("Add model %v", args["name"])
	case models.SchemaOpTypeAddField:
		return fmt.Sprintf("Upsert field on %v", args["model_name"])
	case models.SchemaOpTypeCreateConnection:
		return fmt.Sprintf("Connect %v → %v", args["from"], args["to"])
	case models.SchemaOpTypeUpdateConnection:
		return fmt.Sprintf("Migrate relation %v → %v", args["from"], args["to"])
	case models.SchemaOpTypeRevertSchema:
		return fmt.Sprintf("Revert schema v%v → v%v", args["active_version"], args["target_version"])
	case models.SchemaOpTypeDeleteConnection:
		return fmt.Sprintf("Remove connection %v → %v", args["from"], args["to"])
	case models.SchemaOpTypeUpsertUniqueKey:
		return fmt.Sprintf("Unique index %v on %v", args["name"], args["model_name"])
	case models.SchemaOpTypeDeleteUniqueKey:
		return fmt.Sprintf("Drop unique index %v on %v", args["name"], args["model_name"])
	case models.SchemaOpTypeModelFieldOp:
		return fmt.Sprintf("Field op %v on %v", args["type"], args["model_name"])
	case models.SchemaOpTypeDeleteModel:
		return fmt.Sprintf("Delete model %v", args["model_name"])
	case models.SchemaOpTypeRenameModel:
		return fmt.Sprintf("Rename model %v → %v", args["model_name"], args["new_name"])
	case models.SchemaOpTypeDuplicateModel:
		return fmt.Sprintf("Duplicate model %v → %v", args["model_name"], args["new_name"])
	case models.SchemaOpTypeConvertModel:
		return fmt.Sprintf("Convert model %v", args["model_name"])
	case models.SchemaOpTypeRearrangeField:
		return fmt.Sprintf("Rearrange fields on %v", args["model_name"])
	default:
		return opType
	}
}

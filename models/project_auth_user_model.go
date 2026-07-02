package models

import (
	"strings"
	"time"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/types"
)

const (
	// ExtKeyIsProjectAuthUserModel marks the hidden schema model backed by project DB auth users.
	ExtKeyIsProjectAuthUserModel = "is_project_auth_user_model"
	// ProjectAuthUserModelName is the canonical schema model id (matches ProjectAuthUsersTableName).
	ProjectAuthUserModelName = ProjectAuthUsersTableName
)

// ModelIsProjectAuthUserModel reports whether m is the hidden application end-user schema model.
func ModelIsProjectAuthUserModel(m *ModelType) bool {
	if m == nil {
		return false
	}
	if m.Ext != nil {
		if v, ok := m.Ext[ExtKeyIsProjectAuthUserModel].(bool); ok && v {
			return true
		}
	}
	return strings.EqualFold(strings.TrimSpace(m.Name), ProjectAuthUserModelName) && m.SystemGenerated
}

// ModelNameIsReservedProjectAuthUser reports whether name is reserved for the hidden auth user model.
func ModelNameIsReservedProjectAuthUser(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), ProjectAuthUserModelName)
}

// DefaultProjectAuthUserModel returns the canonical hidden app-user schema descriptor.
// Fields mirror safe public columns on the project DB users table (no secret / google_sub).
func DefaultProjectAuthUserModel() *ModelType {
	text := func(id, label string) *FieldInfo {
		return &FieldInfo{
			Identifier:      id,
			Label:           label,
			InputType:       _const.StringInput,
			FieldType:       _const.TextField,
			SystemGenerated: true,
		}
	}
	return &ModelType{
		Name:            ProjectAuthUserModelName,
		Description:     "Application end-users (hidden system model for relations)",
		SystemGenerated: true,
		Ext: map[string]interface{}{
			ExtKeyIsProjectAuthUserModel: true,
		},
		Fields: []*FieldInfo{
			text("email", "Email"),
			text("phone", "Phone"),
			text("role", "Role"),
			text("provider", "Provider"),
			text("status", "Status"),
			{
				Identifier:      "created_at",
				Label:           "Created At",
				InputType:       _const.StringInput,
				FieldType:       _const.DateField,
				SystemGenerated: true,
			},
			{
				Identifier:      "updated_at",
				Label:           "Updated At",
				InputType:       _const.StringInput,
				FieldType:       _const.DateField,
				SystemGenerated: true,
			},
		},
	}
}

// EnsureProjectAuthUserModelInSchema injects the hidden users model when absent.
// Returns true when a new descriptor was appended (runtime-only; not persisted).
func EnsureProjectAuthUserModelInSchema(schema *ProjectSchema) bool {
	if schema == nil {
		return false
	}
	for _, m := range schema.Models {
		if m == nil {
			continue
		}
		if ModelIsProjectAuthUserModel(m) || ModelNameIsReservedProjectAuthUser(m.Name) {
			if m.Ext == nil {
				m.Ext = map[string]interface{}{}
			}
			m.Ext[ExtKeyIsProjectAuthUserModel] = true
			if !m.SystemGenerated {
				m.SystemGenerated = true
			}
			if len(m.Fields) == 0 {
				m.Fields = DefaultProjectAuthUserModel().Fields
			}
			return false
		}
	}
	schema.Models = append(schema.Models, DefaultProjectAuthUserModel())
	return true
}

// StripProjectAuthUserModelFromPersistedSchema removes the runtime auth users descriptor
// before persisting draft/publish JSON (injection happens again on project load).
func StripProjectAuthUserModelFromPersistedSchema(schema *ProjectSchema) {
	if schema == nil || len(schema.Models) == 0 {
		return
	}
	out := schema.Models[:0]
	for _, m := range schema.Models {
		if m == nil || ModelIsProjectAuthUserModel(m) || ModelNameIsReservedProjectAuthUser(m.Name) {
			continue
		}
		out = append(out, m)
	}
	if len(out) == 0 {
		schema.Models = nil
		return
	}
	schema.Models = out
}

// ProjectAuthUserRowToDocument maps a flat users-table SQL row to DefaultDocumentStructure.
func ProjectAuthUserRowToDocument(model *ModelType, row map[string]interface{}) (*types.DefaultDocumentStructure, error) {
	if row == nil {
		return nil, nil
	}
	id, _ := row["id"].(string)
	if id == "" {
		if b, ok := row["id"].([]byte); ok {
			id = string(b)
		}
	}
	if id == "" {
		return nil, nil
	}
	name := ProjectAuthUserModelName
	if model != nil && model.Name != "" {
		name = model.Name
	}
	data := map[string]interface{}{}
	for _, key := range []string{"email", "phone", "role", "provider"} {
		if v, ok := row[key]; ok && v != nil {
			data[key] = v
		}
	}
	meta := &types.MetaField{}
	if v, ok := row["status"]; ok {
		meta.Status = sqlMetaStringFromAny(v)
	} else if v, ok := row["sys_status"]; ok {
		meta.Status = sqlMetaStringFromAny(v)
	}
	for _, pair := range []struct {
		keys []string
		dst  *string
	}{
		{[]string{"created_at", "sys_created_at"}, &meta.CreatedAt},
		{[]string{"updated_at", "sys_updated_at"}, &meta.UpdatedAt},
	} {
		for _, src := range pair.keys {
			if v, ok := row[src]; ok && v != nil {
				if s, err := formatProjectAuthUserMetaTime(v); err == nil && s != "" {
					*pair.dst = s
					break
				}
			}
		}
	}
	if v, ok := row["status"]; ok && v != nil {
		data["status"] = v
	} else if v, ok := row["sys_status"]; ok {
		data["status"] = v
	}
	for _, src := range []string{"created_at", "updated_at"} {
		if v, ok := row[src]; ok && v != nil {
			if s, err := formatProjectAuthUserMetaTime(v); err == nil && s != "" {
				data[src] = s
			}
		}
	}
	return &types.DefaultDocumentStructure{
		Key:  id,
		ID:   id,
		Type: name,
		Data: data,
		Meta: meta,
	}, nil
}

func sqlMetaStringFromAny(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case []byte:
		return string(t)
	default:
		return ""
	}
}

func formatProjectAuthUserMetaTime(v interface{}) (string, error) {
	switch t := v.(type) {
	case string:
		return t, nil
	case []byte:
		return string(t), nil
	case time.Time:
		return t.UTC().Format(time.RFC3339), nil
	default:
		return "", nil
	}
}

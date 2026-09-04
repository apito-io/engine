package models

type ProjectSchema struct {
	ProjectID string           `bun:"type:uuid,pk" json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"_id,omitempty"`
	Models    []*ModelType     `bun:"rel:has-many,join:project_id=project_id" json:"models,omitempty" firestore:"models,omitempty" bson:"models,omitempty"`
	Functions []*ApitoFunction `bun:"rel:has-many,join:project_id=project_id" json:"functions,omitempty" firestore:"functions,omitempty" bson:"functions,omitempty"`
	// NamingSchemaVersion 0 = legacy; 1 = canonical snake_case model ids (see utility.NamingSchemaVersionV2).
	NamingSchemaVersion int `json:"naming_schema_version,omitempty" firestore:"naming_schema_version,omitempty" bson:"naming_schema_version,omitempty"`
}

type KeyValue struct {
	Key   string `json:"key,omitempty" firestore:"key,omitempty" bson:"key,omitempty"`
	Value string `json:"value,omitempty" firestore:"value,omitempty" bson:"value,omitempty"`
}

type ModelType struct {
	ProjectID       string            `bun:"type:uuid,pk" json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	Name            string            `bun:"type:text,pk" json:"name,omitempty" firestore:"name,omitempty" bson:"name,omitempty"`
	Description     string            `json:"description,omitempty" bson:"description,omitempty"`
	Fields          []*FieldInfo      `json:"fields,omitempty" firestore:"fields,omitempty" bson:"fields,omitempty"`
	Connections     []*ConnectionType `json:"connections,omitempty" firestore:"connections,omitempty" bson:"connections,omitempty"`
	HookIds         []string          `json:"hook_ids,omitempty" firestore:"hook_ids,omitempty" bson:"hook_ids,omitempty"`
	Locals          []string          `json:"locals,omitempty" firestore:"locals,omitempty" bson:"locals,omitempty"`
	RepeatedGroups  []string          `json:"repeated_groups,omitempty" firestore:"locals,omitempty" bson:"repeated_groups,omitempty"`
	SystemGenerated bool              `json:"system_generated,omitempty" firestore:"system_generated,omitempty" bson:"system_generated,omitempty"`
	SinglePage      bool              `json:"single_page,omitempty" firestore:"system_generated,omitempty" bson:"single_page,omitempty"`
	SinglePageUUID  string            `json:"single_page_uuid,omitempty" firestore:"system_generated,omitempty" bson:"single_page_uuid,omitempty"`
	HasConnections  bool              `json:"has_connections,omitempty" firestore:"has_connections,omitempty" bson:"has_connections,omitempty"`
	EnableRevision  bool              `json:"enable_revision,omitempty" firestore:"enable_revision,omitempty" bson:"enable_revision,omitempty"`
	RevisionFilter  []*KeyValue       `json:"revision_filter,omitempty" firestore:"revision_filter,omitempty" bson:"revision_filter,omitempty"`

	// Ext holds opaque extension data populated by the extension layer (e.g. model classification metadata).
	Ext map[string]interface{} `json:"ext,omitempty" firestore:"ext,omitempty" bson:"ext,omitempty"`
}

type Validation struct {
	Required bool `json:"required,omitempty" firestore:"required,omitempty" bson:"required,omitempty"`
	Hide     bool `json:"hide,omitempty" firestore:"hide,omitempty" bson:"hide,omitempty"`
	//AsTitle  bool     `json:"as_title,omitempty" firestore:"as_title,omitempty" bson:"as_title,omitempty"`
	Locals []string `json:"locals,omitempty" firestore:"locals,omitempty" bson:"locals,omitempty"`
	Unique bool     `json:"unique,omitempty" firestore:"unique,omitempty" bson:"unique,omitempty"`

	CharLimit        []uint32  `json:"char_limit,omitempty" firestore:"char_limit,omitempty" bson:"char_limit,omitempty"`
	IntRangeLimit    []uint32  `json:"int_range_limit,omitempty" firestore:"int_range_limit,omitempty" bson:"int_range_limit,omitempty"`
	DoubleRangeLimit []float64 `json:"double_range_limit,omitempty" firestore:"double_range_limit,omitempty" bson:"double_range_limit,omitempty"`

	Placeholder          string        `json:"placeholder,omitempty" firestore:"placeholder,omitempty" bson:"placeholder,omitempty"`
	FixedListElements    []interface{} `json:"fixed_list_elements,omitempty" firestore:"fixed_list_elements,omitempty" bson:"fixed_list_elements,omitempty"`
	FixedListElementType string        `json:"fixed_list_element_type,omitempty" firestore:"fixed_list_element_type,omitempty" bson:"fixed_list_element_type,omitempty"` // if the list element is string or int or float

	IsMultiChoice bool `json:"is_multi_choice,omitempty" firestore:"is_multi_choice,omitempty" bson:"is_multi_choice,omitempty"`
	IsEmail       bool `json:"is_email,omitempty" firestore:"is_email,omitempty" bson:"is_email,omitempty"`
	IsGallery     bool `json:"is_gallery,omitempty" firestore:"is_gallery,omitempty" bson:"is_gallery,omitempty"`
	IsPassword    bool `json:"is_password,omitempty" firestore:"is_gallery,omitempty" bson:"is_password,omitempty"`
	//IsSystemRole      bool      `json:"is_system_role,omitempty" firestore:"is_gallery,omitempty" bson:"is_system_role,omitempty"`
	IsURL bool `json:"is_url,omitempty" firestore:"is_url,omitempty" bson:"is_url,omitempty"`
	// hide the field for the roles
	HideForRoles []string `json:"hide_for_roles,omitempty" firestore:"hide_for_roles,omitempty" bson:"hide_for_roles,omitempty"`
}

type FieldInfo struct {
	Identifier      string       `json:"identifier,omitempty" firestore:"identifier,omitempty" bson:"identifier,omitempty"`
	Description     string       `json:"description,omitempty" firestore:"description,omitempty" bson:"description,omitempty"`
	InputType       string       `json:"input_type,omitempty" firestore:"input_type,omitempty" bson:"input_type,omitempty"`
	FieldType       string       `json:"field_type,omitempty" firestore:"field_type,omitempty" bson:"field_type,omitempty"`
	FieldSubType    string       `json:"field_sub_type,omitempty" bson:"field_sub_type,omitempty"`
	SubFieldInfo    []*FieldInfo `json:"sub_field_info,omitempty" firestore:"modules,omitempty" bson:"sub_field_info,omitempty"`
	Validation      *Validation  `json:"validation,omitempty" firestore:"validation,omitempty" bson:"validation,omitempty"`
	Serial          uint32       `json:"serial,omitempty" firestore:"serial,omitempty" bson:"serial,omitempty"`
	Label           string       `json:"label,omitempty" firestore:"label,omitempty" bson:"label,omitempty"`
	SystemGenerated bool         `json:"system_generated,omitempty" firestore:"system_generated,omitempty" bson:"system_generated,omitempty"`
	//RepeatedGroupIdentifier string       `json:"repeated_group_identifier,omitempty" firestore:"repeated_group_identifier,omitempty" bson:"repeated_group_identifier,omitempty"`
	IsObjectField   bool   `json:"is_object_field,omitempty" firestore:"is_object_field,omitempty" bson:"is_object_field,omitempty"`
	ParentField     string `json:"parent_field,omitempty" firestore:"parent_field,omitempty" bson:"parent_field,omitempty"`
	EnableIndexing  bool   `json:"enable_indexing,omitempty" firestore:"enable_indexing,omitempty" bson:"enable_indexing,omitempty"`
	PluginID        string `json:"plugin_id,omitempty" firestore:"plugin_id,omitempty" bson:"plugin_id,omitempty"`
	PluginFieldType string `json:"plugin_field_type,omitempty" firestore:"plugin_field_type,omitempty" bson:"plugin_field_type,omitempty"`
}

type ConnectionType struct {
	Model    string `json:"model,omitempty" firestore:"model,omitempty" bson:"model,omitempty"`
	Relation string `json:"relation,omitempty" firestore:"relation,omitempty" bson:"relation,omitempty"`
	Type     string `json:"type,omitempty" firestore:"type,omitempty" bson:"type,omitempty"`
	KnownAs  string `json:"known_as,omitempty" firestore:"known_as,omitempty" bson:"known_as,omitempty"`
}

// NormalizeProjectSchemaConnectionTypes maps legacy pro draft "reverse" direction labels to "backward".
// Connection direction is always forward/backward; reverse_connection_type GraphQL args are cardinalities only.
func NormalizeProjectSchemaConnectionTypes(schema *ProjectSchema) {
	if schema == nil {
		return
	}
	for _, m := range schema.Models {
		if m == nil {
			continue
		}
		for _, c := range m.Connections {
			if c != nil && c.Type == "reverse" {
				c.Type = "backward"
			}
		}
	}
}

// DedupeProjectSchemaFields removes duplicate root-level fields per model (last wins).
func DedupeProjectSchemaFields(schema *ProjectSchema) {
	if schema == nil {
		return
	}
	NormalizeProjectSchemaConnectionTypes(schema)
	for _, m := range schema.Models {
		if m == nil {
			continue
		}
		m.Fields = DedupeFieldsByIdentifier(m.Fields)
	}
}

// DedupeFieldsByIdentifier keeps one FieldInfo per identifier (last occurrence wins).
func DedupeFieldsByIdentifier(fields []*FieldInfo) []*FieldInfo {
	seen := make(map[string]int)
	out := make([]*FieldInfo, 0, len(fields))
	for _, f := range fields {
		if f == nil || f.Identifier == "" {
			continue
		}
		if idx, ok := seen[f.Identifier]; ok {
			out[idx] = f
			continue
		}
		seen[f.Identifier] = len(out)
		out = append(out, f)
	}
	return out
}

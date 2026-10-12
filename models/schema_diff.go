package models

// Schema change impact levels for diff and publish planning.
const (
	SchemaImpactMetadataOnly = "metadata_only"
	SchemaImpactPhysicalDDL  = "physical_ddl"
	SchemaImpactProjectData  = "project_data"
	SchemaImpactSyncOnly     = "sync_only"
	SchemaImpactNoOp         = "no_op"
)

// SchemaDiff is the computed diff between active and draft schemas.
type SchemaDiff struct {
	ModelsAdded         int                `json:"models_added"`
	ModelsRemoved       int                `json:"models_removed"`
	ModelsModified      int                `json:"models_modified"`
	FieldsAdded         int                `json:"fields_added"`
	FieldsRemoved       int                `json:"fields_removed"`
	FieldsModified      int                `json:"fields_modified"`
	ConnectionsAdded    int                `json:"connections_added"`
	ConnectionsRemoved  int                `json:"connections_removed"`
	RequiresPhysicalDDL bool               `json:"requires_physical_ddl"`
	Destructive         bool               `json:"destructive"`
	Warnings            []string           `json:"warnings,omitempty"`
	ModelChanges        []*SchemaModelDiff `json:"model_changes,omitempty"`
}

// SchemaFieldChange describes one in-place field mutation.
type SchemaFieldChange struct {
	Identifier  string   `json:"identifier"`
	ParentField string   `json:"parent_field,omitempty"`
	Label       string   `json:"label,omitempty"`
	Impact      string   `json:"impact"`
	Changes     []string `json:"changes,omitempty"`
}

// SchemaModelDiff describes changes within one model.
type SchemaModelDiff struct {
	ModelName           string                    `json:"model_name"`
	Added               bool                      `json:"added,omitempty"`
	Removed             bool                      `json:"removed,omitempty"`
	RequiresPhysicalDDL bool                      `json:"requires_physical_ddl,omitempty"`
	FieldsAdded         []*FieldInfo              `json:"fields_added,omitempty"`
	FieldsRemoved       []*FieldInfo              `json:"fields_removed,omitempty"`
	FieldsModified      []*SchemaFieldChange      `json:"fields_modified,omitempty"`
	ConnectionsAdded    []*ConnectionType         `json:"connections_added,omitempty"`
	ConnectionsRemoved  []*ConnectionType         `json:"connections_removed,omitempty"`
	ConnectionsModified []*SchemaConnectionChange `json:"connections_modified,omitempty"`
	UniqueKeysAdded     []*UniqueKey              `json:"unique_keys_added,omitempty"`
	UniqueKeysRemoved   []*UniqueKey              `json:"unique_keys_removed,omitempty"`
	UniqueKeysModified  []*SchemaUniqueKeyChange  `json:"unique_keys_modified,omitempty"`
}

// SchemaConnectionChange is one cardinality edit on a retained edge.
type SchemaConnectionChange struct {
	Model              string `json:"model"`
	KnownAs            string `json:"known_as,omitempty"`
	BeforeRelation     string `json:"before_relation"`
	AfterRelation      string `json:"after_relation"`
	PeerBeforeRelation string `json:"peer_before_relation,omitempty"`
	PeerAfterRelation  string `json:"peer_after_relation,omitempty"`
	Impact             string `json:"impact,omitempty"`
}

// SchemaUniqueKeyChange describes an in-place composite unique index edit.
type SchemaUniqueKeyChange struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns,omitempty"`
	Impact  string   `json:"impact"`
	Changes []string `json:"changes,omitempty"`
}

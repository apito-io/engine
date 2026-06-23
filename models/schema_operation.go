package models

import (
	"encoding/json"
)

// Schema operation ledger statuses.
const (
	SchemaOpStatusPending      = "pending"
	SchemaOpStatusApplying     = "applying"
	SchemaOpStatusCommitted    = "committed"
	SchemaOpStatusCompensating = "compensating"
	SchemaOpStatusFailed       = "failed"
	SchemaOpStatusNeedsRepair  = "needs_repair"
)

// Schema operation types (GraphQL mutation names / internal keys).
const (
	SchemaOpTypeCreateModel         = "create_model"
	SchemaOpTypeAddField            = "add_field"
	SchemaOpTypeModelFieldOp        = "model_field_operation"
	SchemaOpTypeCreateConnection    = "create_connection"
	SchemaOpTypeDeleteConnection    = "delete_connection"
	SchemaOpTypeDeleteModel         = "delete_model"
	SchemaOpTypeRenameModel         = "rename_model"
	SchemaOpTypeDuplicateModel      = "duplicate_model"
	SchemaOpTypeConvertModel        = "convert_model"
	SchemaOpTypeUpdateModel         = "update_model"
	SchemaOpTypeRearrangeField      = "rearrange_field"
	SchemaOpTypePublishChangeset    = "publish_schema_changeset"
)

// SchemaOperationStepState is per-participant step state in steps_json.
const (
	SchemaOpStepPending     = "pending"
	SchemaOpStepSucceeded   = "succeeded"
	SchemaOpStepFailed      = "failed"
	SchemaOpStepCompensated = "compensated"
)

// SchemaOperationStep records one orchestration step (base DB, scoped target, system, cache).
type SchemaOperationStep struct {
	Key     string `json:"key"`
	Scope   string `json:"scope,omitempty"` // empty = base project; otherwise scope_key (e.g. tenant id)
	State   string `json:"state"`
	Error   string `json:"error,omitempty"`
	Updated string `json:"updated_at,omitempty"`
}

// SchemaOperation is the persisted saga log for a schema mutation.
type SchemaOperation struct {
	ORMBase `bun:"table:schema_operations,alias:schema_op"`

	ID              string `bun:"id,pk,type:uuid"`
	ProjectID       string `bun:"project_id,type:uuid,notnull"`
	OperationType   string `bun:"operation_type,type:text,notnull"`
	Status          string `bun:"status,type:text,notnull"`
	RequestJSON     string `bun:"request_json,type:text"`
	BeforeSchemaJSON string `bun:"before_schema_json,type:text"`
	StepsJSON       string `bun:"steps_json,type:text"`
	Error           string `bun:"error,type:text"`
	AttemptCount    int    `bun:"attempt_count,type:int,notnull,default:0"`
	CreatedAt       string `bun:"created_at,type:timestamp,notnull"`
	UpdatedAt       string `bun:"updated_at,type:timestamp,notnull"`
}

// Steps decodes StepsJSON into structured steps.
func (o *SchemaOperation) Steps() ([]SchemaOperationStep, error) {
	if o == nil || o.StepsJSON == "" {
		return nil, nil
	}
	var steps []SchemaOperationStep
	if err := decodeJSON(o.StepsJSON, &steps); err != nil {
		return nil, err
	}
	return steps, nil
}

// SetSteps encodes steps into StepsJSON.
func (o *SchemaOperation) SetSteps(steps []SchemaOperationStep) error {
	if o == nil {
		return nil
	}
	b, err := encodeJSON(steps)
	if err != nil {
		return err
	}
	o.StepsJSON = string(b)
	return nil
}

func decodeJSON(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}

func encodeJSON(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

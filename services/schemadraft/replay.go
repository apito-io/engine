package schemadraft

import (
	"github.com/apito-io/engine/models"
)

// Replay applies staged file operations with the same replay Cloud uses.
// create_model and add_model are one step. The local preview drops an unconnected users model.
func Replay(live *models.ProjectSchema, ops []Op) (*models.ProjectSchema, error) {
	converted := make([]*Operation, 0, len(ops))
	for _, op := range ops {
		converted = append(converted, op.asOperation())
	}
	draft, err := ReplayOperations(live, converted)
	if err != nil {
		return nil, err
	}
	models.StripProjectAuthUserModelFromPersistedSchema(draft)
	return draft, nil
}

func (op Op) asOperation() *Operation {
	payload := op.Args
	if payload == nil {
		payload = map[string]interface{}{}
	}
	typ := op.Type
	switch typ {
	case "create_model":
		typ = "add_model"
	case "remove_connection":
		typ = "delete_connection"
	}
	return &Operation{
		ID:        op.ID,
		Type:      typ,
		ModelName: firstStr(payload, "model_name", "name", "model"),
		FieldName: firstStr(payload, "field_name", "field_identifier", "identifier", "field"),
		Payload:   payload,
		CreatedAt: op.CreatedAt,
	}
}

func firstStr(args map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := args[key].(string); ok && v != "" {
			return v
		}
	}
	return ""
}

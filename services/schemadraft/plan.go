package schemadraft

import "fmt"

// Plan builds the pending execution rows the Schema Changes table groups by operation.
// Public rows stay pending until synchronous publish. They are not a Cloud repair ledger.
func Plan(projectID, changesetID string, ops []Op) []map[string]interface{} {
	rows := make([]map[string]interface{}, 0, len(ops)*2)
	seq := 0
	for _, op := range ops {
		action, impact, ddl, systemMsg, projectMsg := describe(op)
		seq++
		rows = append(rows, record(projectID, changesetID, op, seq, "system", action, impact, false, systemMsg, ""))
		seq++
		rows = append(rows, record(projectID, changesetID, op, seq, "project", action, impact, ddl, "", projectMsg))
	}
	return rows
}

func record(projectID, changesetID string, op Op, seq int, scope, action, impact string, ddl bool, systemMsg, projectMsg string) map[string]interface{} {
	remote := "not_applicable"
	return map[string]interface{}{
		"id":              fmt.Sprintf("%s-%s", op.ID, scope),
		"project_id":      projectID,
		"changeset_id":    changesetID,
		"operation_id":    op.ID,
		"sequence":        seq,
		"scope_kind":      scope,
		"scope_key":       scope,
		"target_kind":     "schema",
		"target_name":     firstStr(op.Args, "model_name", "name", "from"),
		"action_key":      action,
		"impact":          impact,
		"requires_ddl":    ddl,
		"retryable":       false,
		"system_message":  systemMsg,
		"project_message": projectMsg,
		"local_status":    "pending",
		"remote_status":   remote,
		"status":          "pending",
		"error":           "",
		"attempt_count":   0,
		"created_at":      op.CreatedAt,
		"updated_at":      op.CreatedAt,
	}
}

func describe(op Op) (action, impact string, ddl bool, systemMsg, projectMsg string) {
	modelName := firstStr(op.Args, "model_name", "name")
	switch op.Type {
	case "create_model", "add_model":
		return "add_model", "physical_ddl", true,
			"Add model " + modelName,
			"Create table for " + modelName
	case "delete_model":
		return "delete_model", "physical_ddl", true,
			"Remove model " + modelName,
			"Drop table for " + modelName
	case "rename_model":
		return "rename_model", "physical_ddl", true,
			"Rename model " + modelName + " to " + StringArg(op.Args, "new_name"),
			"Rename table " + modelName
	case "add_field":
		field := fieldLabel(op)
		return "add_field", "physical_ddl", true,
			fmt.Sprintf("field %s added to model %s", field, modelName),
			"Add column " + field + " on " + modelName
	case "model_field_operation":
		field := firstStr(op.Args, "field_name", "field")
		switch StringArg(op.Args, "type") {
		case "delete":
			return "remove_field", "physical_ddl", true,
				"Remove field " + field + " on " + modelName,
				"Drop column " + field
		default:
			return "update_field", "metadata_only", false,
				"Update field " + field + " on " + modelName,
				"No database migration"
		}
	case "rearrange_field":
		return "reorder_field", "metadata_only", false,
			"Reorder field " + firstStr(op.Args, "field_name", "dragged_field"),
			"No database migration"
	case "create_connection":
		return "add_connection", "metadata_only", false,
			fmt.Sprintf("Connect %s to %s", StringArg(op.Args, "from"), StringArg(op.Args, "to")),
			"No database migration"
	case "delete_connection", "remove_connection":
		return "remove_connection", "metadata_only", false,
			fmt.Sprintf("Remove connection %s to %s", StringArg(op.Args, "from"), StringArg(op.Args, "to")),
			"No database migration"
	case "update_connection":
		return "update_field", "physical_ddl", true,
			fmt.Sprintf("Migrate relation %s to %s", StringArg(op.Args, "from"), StringArg(op.Args, "to")),
			"DB migration on publish"
	default:
		label := op.Type
		return label, "metadata_only", false, label, "No database migration"
	}
}

func fieldLabel(op Op) string {
	if label := StringArg(op.Args, "field_label"); label != "" {
		return label
	}
	return StringArg(op.Args, "field")
}

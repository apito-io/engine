package mcp

import "strings"

const (
	accessRead        = "read"
	accessWrite       = "write"
	accessDestructive = "destructive"
	accessGuide       = "guide"
	accessScope       = "scope"
)

// Tool is one MCP tool exposed by the engine adapter.
type Tool struct {
	Name        string
	Description string
	Access      string
	ProOnly     bool
}

func (t Tool) schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id":                     map[string]any{"type": "string"},
			"tenant_id":                      map[string]any{"type": "string"},
			"scope_lease":                    map[string]any{"type": "string"},
			"preparation_id":                 map[string]any{"type": "string"},
			"confirm_destructive":            map[string]any{"type": "boolean"},
			"acknowledge_permanent_deletion": map[string]any{"type": "boolean"},
		},
		"additionalProperties": true,
	}
}

func Catalog() []Tool {
	names := []string{
		"prepare_project_scope", "confirm_project_scope", "get_project_scope",
		"create_model", "add_field", "update_field", "rename_field", "delete_field",
		"update_model", "rename_model", "delete_model",
		"get_schema_migration_guide", "get_schema_versioning_status", "get_schema_preview",
		"get_effective_schema", "get_schema_change_plan", "summarize_schema_draft_for_review",
		"list_models", "get_model_schema", "get_model_physical_health", "get_project_physical_health",
		"get_saas_model_guide", "reconcile_common_models", "get_project_context", "get_relation_graph",
		"get_project_query_structure", "get_public_graphql_model_map", "probe_public_document",
		"inspect_access_token", "get_field_design_guide",
		"add_relation", "preview_relation_transition", "migrate_relation", "delete_relation",
		"upsert_unique_key", "delete_unique_key",
		"upsert_data", "get_data", "delete_data", "duplicate_data",
		"list_tenants", "search_tenants", "create_tenant", "update_tenant", "delete_tenant", "restore_tenant",
		"generate_tenant_token", "search_tenant_by_domain",
		"search_app_users", "create_app_user", "update_app_user", "delete_app_user",
		"reset_app_user_password", "login_app_user", "google_oauth_state", "login_app_user_google",
		"get_schema_diff", "list_schema_versions", "list_schema_change_events", "discard_schema_draft",
		"publish_schema",
		"list_roles", "get_permissions_catalog", "upsert_role", "duplicate_role", "delete_role",
		"list_plans", "upsert_plan", "delete_plan", "get_project_settings", "update_project_settings",
		"get_client_runtime", "update_client_runtime",
		"list_api_keys", "create_api_key", "delete_api_key",
		"get_auth_settings", "update_auth_settings", "get_storage_settings", "update_storage_settings",
		"list_team_members", "update_team_members",
		"list_webhooks", "create_webhook", "delete_webhook",
		"list_plugins", "configure_plugin", "remove_plugin",
		"list_functions", "upsert_function", "delete_function", "test_function_draft", "deploy_function",
		"execute_function", "list_function_revisions", "list_function_deployments", "rollback_function",
		"list_media", "upload_media_from_url", "delete_media",
		"list_data", "connect_relation", "disconnect_relation",
		"get_model_document_counts", "list_document_revisions", "reorder_fields",
		"get_saas_auth_guide",
		"search_system_logs", "get_system_log", "get_system_trace", "summarize_system_logs", "get_log_store_health",
	}
	out := make([]Tool, 0, len(names))
	for _, name := range names {
		out = append(out, Tool{
			Name:        name,
			Description: toolDescription(name),
			Access:      classify(name),
			ProOnly:     proOnly(name),
		})
	}
	return out
}

func classify(name string) string {
	switch name {
	case "prepare_project_scope", "confirm_project_scope", "get_project_scope":
		return accessScope
	case "get_schema_migration_guide", "get_saas_model_guide", "get_field_design_guide",
		"get_project_query_structure", "get_saas_auth_guide", "inspect_access_token":
		return accessGuide
	}
	if strings.HasPrefix(name, "delete_") || strings.HasPrefix(name, "remove_") ||
		strings.HasPrefix(name, "discard_") || strings.HasPrefix(name, "rollback_") ||
		strings.HasPrefix(name, "disconnect_") {
		return accessDestructive
	}
	if strings.HasPrefix(name, "get_") || strings.HasPrefix(name, "list_") || strings.HasPrefix(name, "search_") ||
		name == "preview_relation_transition" || name == "probe_public_document" ||
		name == "google_oauth_state" || name == "login_app_user" || name == "login_app_user_google" ||
		name == "summarize_schema_draft_for_review" || name == "summarize_system_logs" {
		return accessRead
	}
	return accessWrite
}

func proOnly(name string) bool {
	switch name {
	case "list_tenants", "search_tenants", "create_tenant", "update_tenant", "delete_tenant", "restore_tenant",
		"generate_tenant_token", "search_tenant_by_domain",
		"get_schema_diff", "list_schema_versions", "list_schema_change_events", "discard_schema_draft",
		"search_system_logs", "get_system_log", "get_system_trace", "summarize_system_logs", "get_log_store_health":
		return true
	default:
		return false
	}
}

func toolDescription(name string) string {
	return "Apito MCP tool " + name + ". Schema changes are staged; publish stays in the Console."
}

func guides() map[string]string {
	return map[string]string{
		"get_schema_migration_guide":  "Stage schema changes with MCP. Review the draft in Console and publish there. MCP does not publish.",
		"get_saas_model_guide":        "SaaS tenant rows use platform tenant tools. Do not create or delete tenants through get_data.",
		"get_field_design_guide":      "Use explicit field types. Relation cardinality changes go through preview_relation_transition and migrate_relation.",
		"get_project_query_structure": "Public root operations are camelCase. Nested selection and connect keys stay snake_case.",
		"get_saas_auth_guide":         "App users authenticate through the project auth tools. Operator access uses the MCP OAuth grant.",
		"inspect_access_token":        "The remote MCP credential is resource-bound. The underlying operator grant stays on the server.",
	}
}

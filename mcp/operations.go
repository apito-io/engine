package mcp

import "strings"

// operationQuery returns the system GraphQL document for a ported tool.
// Guides and scope tools are handled in the adapter and have no document.
func operationQuery(name string) (string, bool) {
	query, ok := operations[name]
	return query, ok
}

func variablesFor(query string, args map[string]any) map[string]any {
	if args == nil {
		return map[string]any{}
	}
	out := map[string]any{}
	for key, value := range args {
		if strings.Contains(query, "$"+key) {
			out[key] = value
		}
	}
	return out
}

func aliasArgs(name string, args map[string]any) map[string]any {
	if args == nil {
		args = map[string]any{}
	}
	out := map[string]any{}
	for key, value := range args {
		out[key] = value
	}
	switch name {
	case "get_data", "list_data":
		if _, ok := out["model"]; !ok {
			if model, ok := out["model_name"].(string); ok {
				out["model"] = model
			}
		}
	case "upsert_data", "delete_data", "duplicate_data", "connect_relation", "disconnect_relation":
		if _, ok := out["model_name"]; !ok {
			if model, ok := out["model"].(string); ok {
				out["model_name"] = model
			}
		}
		if _, ok := out["_id"]; !ok {
			if id, ok := out["id"].(string); ok {
				out["_id"] = id
			}
		}
		if _, ok := out["payload"]; !ok {
			if data, ok := out["data"]; ok {
				out["payload"] = data
			}
		}
	case "probe_public_document":
		if _, ok := out["model_name"]; !ok {
			if model, ok := out["model"].(string); ok {
				out["model_name"] = model
			}
		}
	case "search_system_logs":
		if _, ok := out["text"]; !ok {
			if q, ok := out["q"].(string); ok {
				out["text"] = q
			}
		}
	case "reorder_fields":
		if _, ok := out["new_position"]; !ok {
			if serial, ok := out["serial"]; ok {
				out["new_position"] = serial
			}
		}
		if _, ok := out["move_type"]; !ok {
			out["move_type"] = "reorder"
		}
	}
	return out
}

var operations = map[string]string{
	"list_models":                       `query($model_name: String) { projectModelsInfo(model_name: $model_name) { name } }`,
	"get_model_schema":                  `query($model_name: String) { projectModelsInfo(model_name: $model_name) { name fields { identifier field_type } } }`,
	"get_schema_versioning_status":      `query { schemaVersioningStatus { enabled has_draft changeset_id pending_operations } }`,
	"get_schema_preview":                `query($source: String, $version: Int) { schemaPreview(source: $source, version: $version) }`,
	"get_effective_schema":              `query { schemaPreview(source: "effective") }`,
	"get_schema_change_plan":            `query($changeset_id: String, $version: Int, $limit: Int, $offset: Int) { schemaChangeExecutionRecords(changeset_id: $changeset_id, version: $version, limit: $limit, offset: $offset) { id target_name action_key status error } }`,
	"summarize_schema_draft_for_review": `query($changeset_id: String, $limit: Int) { schemaChangeExecutionRecords(changeset_id: $changeset_id, limit: $limit) { id target_name action_key status error } }`,
	"get_schema_diff":                   `query($changeset_id: String) { schemaDiff(changeset_id: $changeset_id) }`,
	"list_schema_versions":              `query($limit: Int, $offset: Int) { schemaVersions(limit: $limit, offset: $offset) { version published_at published_by message changeset_id } }`,
	"list_schema_change_events":         `query($changeset_id: String) { schemaChangeEvents(changeset_id: $changeset_id) { event_type actor_id created_at } }`,
	"discard_schema_draft":              `mutation($changeset_id: String!) { discardSchemaDraft(changeset_id: $changeset_id) }`,
	"create_model":                      `mutation($model_name: String!, $single_record: Boolean, $is_common_model: Boolean) { addModelToProject(name: $model_name, single_record: $single_record, is_common_model: $is_common_model) { name } }`,
	"add_field":                         `mutation($model_name: String!, $field_label: String!, $field_type: FIELD_TYPE_ENUM, $field_sub_type: FIELD_SUB_TYPE_ENUM, $input_type: INPUT_TYPE_ENUM, $parent_field: String, $is_object_field: Boolean, $serial: Int, $field_description: String, $validation: module_validation_payload) { upsertFieldToModel(model_name: $model_name, field_label: $field_label, field_type: $field_type, field_sub_type: $field_sub_type, input_type: $input_type, parent_field: $parent_field, is_object_field: $is_object_field, serial: $serial, field_description: $field_description, validation: $validation) { identifier } }`,
	"update_field":                      `mutation($model_name: String!, $field_label: String!, $field_type: FIELD_TYPE_ENUM, $field_sub_type: FIELD_SUB_TYPE_ENUM, $input_type: INPUT_TYPE_ENUM, $parent_field: String, $field_description: String, $validation: module_validation_payload) { upsertFieldToModel(model_name: $model_name, field_label: $field_label, field_type: $field_type, field_sub_type: $field_sub_type, input_type: $input_type, parent_field: $parent_field, is_update: true, field_description: $field_description, validation: $validation) { identifier } }`,
	"rename_field":                      `mutation($model_name: String!, $field_name: String!, $new_name: String, $parent_field: String) { modelFieldOperation(type: rename, model_name: $model_name, field_name: $field_name, new_name: $new_name, parent_field: $parent_field) { identifier } }`,
	"delete_field":                      `mutation($model_name: String!, $field_name: String!, $parent_field: String) { modelFieldOperation(type: delete, model_name: $model_name, field_name: $field_name, parent_field: $parent_field) { identifier } }`,
	"update_model":                      `mutation($model_name: String!, $single_page_model: Boolean, $is_common_model: Boolean, $common_model_migration: String) { updateModel(type: update, model_name: $model_name, single_page_model: $single_page_model, is_common_model: $is_common_model, common_model_migration: $common_model_migration) { name } }`,
	"rename_model":                      `mutation($model_name: String!, $new_name: String) { updateModel(type: rename, model_name: $model_name, new_name: $new_name) { name } }`,
	"delete_model":                      `mutation($model_name: String!) { updateModel(type: delete, model_name: $model_name) { name } }`,
	"add_relation":                      `mutation($from_model: String!, $to_model: String!, $forward_connection_type: RELATION_TYPE_ENUM!, $reverse_connection_type: RELATION_TYPE_ENUM!, $known_as: String) { upsertConnectionToModel(from: $from_model, to: $to_model, forward_connection_type: $forward_connection_type, reverse_connection_type: $reverse_connection_type, known_as: $known_as) { model known_as } }`,
	"preview_relation_transition":       `query($from_model: String!, $to_model: String!, $known_as: String, $from_relation: String, $to_relation: String, $target_from_relation: String!, $target_to_relation: String!) { previewSchemaRelationTransition(from_model: $from_model, to_model: $to_model, known_as: $known_as, from_relation: $from_relation, to_relation: $to_relation, target_from_relation: $target_from_relation, target_to_relation: $target_to_relation) { plan_hash blocked safety } }`,
	"migrate_relation":                  `mutation($from_model: String!, $to_model: String!, $known_as: String, $from_relation: String, $to_relation: String, $target_from_relation: String!, $target_to_relation: String!, $plan_hash: String!, $acknowledgement: String) { stageRelationTransition(from_model: $from_model, to_model: $to_model, known_as: $known_as, from_relation: $from_relation, to_relation: $to_relation, target_from_relation: $target_from_relation, target_to_relation: $target_to_relation, plan_hash: $plan_hash, acknowledgement: $acknowledgement) { staged plan_hash message } }`,
	"delete_relation":                   `mutation($from_model: String!, $to_model: String!, $known_as: String) { deleteConnectionFromModel(from: $from_model, to: $to_model, known_as: $known_as) { model known_as } }`,
	"upsert_unique_key":                 `mutation($model_name: String!, $name: String!, $columns: [String!]!) { upsertUniqueKey(model_name: $model_name, name: $name, columns: $columns) { name staged changeset_id } }`,
	"delete_unique_key":                 `mutation($model_name: String!, $name: String!) { deleteUniqueKey(model_name: $model_name, name: $name) { name staged } }`,
	"reorder_fields":                    `mutation($model_name: String!, $field_name: String!, $new_position: Int!, $move_type: String!, $parent_id: String) { rearrangeSerialOfField(model_name: $model_name, field_name: $field_name, new_position: $new_position, move_type: $move_type, parent_id: $parent_id) { name } }`,
	"get_model_physical_health":         `query($model_name: String!) { modelPhysicalHealth(model_name: $model_name) { model_name table_exists missing_columns extra_columns warnings } }`,
	"get_project_physical_health":       `query($model_names: [String!]) { projectPhysicalHealth(model_names: $model_names) { model_name table_exists warnings } }`,
	"reconcile_common_models":           `mutation($model_name: String) { reconcileCommonModels(model_name: $model_name) { promoted shadows_fixed orphans_removed tables_reduced } }`,
	"get_project_context":               `query { currentProject { id name } }`,
	"get_relation_graph":                `query { projectSchemaRelationGraph }`,
	"get_public_graphql_model_map":      `query { projectModelsInfo { name } }`,
	"probe_public_document":             `query($model_name: String, $id: String!) { getSingleData(model: $model_name, _id: $id) { id } }`,
	"get_data":                          `query($model: String!, $limit: Int, $page: Int, $search: String, $where: JSON) { getModelData(model: $model, limit: $limit, page: $page, search: $search, where: $where) { count } }`,
	"list_data":                         `query($model: String!, $limit: Int, $page: Int, $search: String, $where: JSON) { getModelData(model: $model, limit: $limit, page: $page, search: $search, where: $where) { count } }`,
	"delete_data":                       `mutation($model_name: String!, $_id: String) { deleteModelData(model_name: $model_name, _id: $_id) { id } }`,
	"upsert_data":                       `mutation($model_name: String!, $_id: String, $payload: JSON, $status: String, $connect: JSON, $disconnect: JSON) { upsertModelData(model_name: $model_name, _id: $_id, payload: $payload, status: $status, connect: $connect, disconnect: $disconnect) { id } }`,
	"duplicate_data":                    `mutation($model_name: String!, $_id: String) { duplicateModelData(model_name: $model_name, _id: $_id) { id } }`,
	"connect_relation":                  `mutation($model_name: String!, $_id: String, $connect: JSON) { upsertModelData(model_name: $model_name, _id: $_id, connect: $connect) { id } }`,
	"disconnect_relation":               `mutation($model_name: String!, $_id: String, $disconnect: JSON) { upsertModelData(model_name: $model_name, _id: $_id, disconnect: $disconnect) { id } }`,
	"get_model_document_counts":         `query($models: [String]) { modelDocumentCounts(models: $models) { model count } }`,
	"list_document_revisions":           `query($_id: String!, $model: String) { listSingleModelRevisionData(_id: $_id, model: $model) { id revision_at status } }`,
	"list_tenants":                      `query { getTenants { tenants { id name } } }`,
	"search_tenants":                    `query($project_id: String!, $limit: Int, $offset: Int, $q: String, $status: String) { searchTenants(project_id: $project_id, limit: $limit, offset: $offset, q: $q, status: $status) { count } }`,
	"create_tenant":                     `mutation($name: String!, $data: String, $domain: String, $plan_tier: String) { createTenant(name: $name, data: $data, domain: $domain, plan_tier: $plan_tier) { id name } }`,
	"update_tenant":                     `mutation($tenant_id: String!, $name: String, $data: String, $domain: String, $plan_tier: String) { updateTenant(tenant_id: $tenant_id, name: $name, data: $data, domain: $domain, plan_tier: $plan_tier) { id } }`,
	"delete_tenant":                     `mutation($tenant_id: String!) { deleteTenant(tenant_id: $tenant_id) }`,
	"restore_tenant":                    `mutation($tenant_id: String!) { restoreTenant(tenant_id: $tenant_id) }`,
	"generate_tenant_token":             `mutation($tenant_id: String!, $duration: String!, $role: String) { generateTenantToken(tenant_id: $tenant_id, duration: $duration, role: $role) { tenant_id role } }`,
	"search_tenant_by_domain":           `query($project_id: String!, $domain: String!) { searchTenantsByDomain(project_id: $project_id, domain: $domain) { tenant { id name } } }`,
	"search_app_users":                  `query($project_id: String!, $limit: Int, $offset: Int, $tenant_id: String, $q: String) { searchUsers(project_id: $project_id, limit: $limit, offset: $offset, tenant_id: $tenant_id, q: $q) { count users { id email role tenant_id } } }`,
	"create_app_user":                   `mutation($project_id: String!, $password: String!, $email: String, $phone: String, $role: String, $username: String, $tenant_id: String) { createUser(project_id: $project_id, password: $password, email: $email, phone: $phone, role: $role, username: $username, tenant_id: $tenant_id) { id email role } }`,
	"update_app_user":                   `mutation($user_id: String!, $email: String, $phone: String, $role: String, $username: String, $tenant_id: String) { updateUser(user_id: $user_id, email: $email, phone: $phone, role: $role, username: $username, tenant_id: $tenant_id) { id email role } }`,
	"delete_app_user":                   `mutation($user_id: String!) { deleteUser(user_id: $user_id) }`,
	"reset_app_user_password":           `mutation($user_id: String!, $password: String!) { resetUserPassword(user_id: $user_id, password: $password) }`,
	"login_app_user":                    `query($project_id: String!, $tenant_id: String, $email: String, $phone: String, $password: String) { loginUser(project_id: $project_id, tenant_id: $tenant_id, email: $email, phone: $phone, password: $password) { user { id email role } } }`,
	"google_oauth_state":                `query($project_id: String!) { googleOAuthState(project_id: $project_id) { state } }`,
	"login_app_user_google":             `query($project_id: String!, $tenant_id: String, $code: String, $state: String, $id_token: String) { loginUser(project_id: $project_id, tenant_id: $tenant_id, code: $code, state: $state, id_token: $id_token) { user { id email role } } }`,
	"list_roles":                        `query { currentProject { id roles } }`,
	"get_permissions_catalog":           `query { listPermissionsAndScopes { permissions models functions } }`,
	"upsert_role":                       `mutation($name: String!, $is_admin: Boolean, $logic_executions: [String], $api_permissions: JSON) { upsertRoleToProject(name: $name, is_admin: $is_admin, logic_executions: $logic_executions, api_permissions: $api_permissions) { is_admin } }`,
	"duplicate_role":                    `mutation($source_role: String!, $new_name: String!) { duplicateRoleInProject(source_role: $source_role, new_name: $new_name) { is_admin } }`,
	"delete_role":                       `mutation($role: String!) { deleteRoleFromProject(role: $role) { message } }`,
	"list_plans":                        `query { getProjectPlans { id name description api_permissions logic_executions quotas system_generated currency price_monthly play_product_id play_base_plan_id paddle_price_id prices provider_products } }`,
	"upsert_plan":                       `mutation($id: String!, $name: String, $description: String, $api_permissions: JSON, $logic_executions: [String], $quotas: JSON, $currency: String, $price_monthly: Float, $play_product_id: String, $play_base_plan_id: String, $paddle_price_id: String, $prices: JSON, $provider_products: JSON) { upsertPlanToProject(id: $id, name: $name, description: $description, api_permissions: $api_permissions, logic_executions: $logic_executions, quotas: $quotas, currency: $currency, price_monthly: $price_monthly, play_product_id: $play_product_id, play_base_plan_id: $play_base_plan_id, paddle_price_id: $paddle_price_id, prices: $prices, provider_products: $provider_products) { id name api_permissions logic_executions quotas currency price_monthly prices provider_products } }`,
	"delete_plan":                       `mutation($id: String!) { deletePlanFromProject(id: $id) { id name } }`,
	"get_project_settings":              `query { currentProject { id name description } }`,
	"update_project_settings":           `mutation($name: String, $description: String, $client_runtime: UpdateClientRuntimeInput) { updateProject(name: $name, description: $description, settings: { client_runtime: $client_runtime }) { id name settings { client_runtime { android { min_version_name min_build_number force_update store_url message_en message_bn maintenance maintenance_hours extras } ios { min_version_name min_build_number force_update store_url message_en message_bn maintenance maintenance_hours extras } web { min_version_name min_build_number force_update store_url message_en message_bn maintenance maintenance_hours extras } } } } }`,
	"get_client_runtime":                `query($platform: String!) { clientRuntime(platform: $platform) { platform min_version_name min_build_number force_update store_url message_en message_bn maintenance maintenance_hours extras } }`,
	"update_client_runtime":             `mutation($client_runtime: UpdateClientRuntimeInput!) { updateProject(settings: { client_runtime: $client_runtime }) { id settings { client_runtime { android { min_version_name force_update maintenance store_url extras } ios { min_version_name force_update maintenance } web { min_version_name force_update maintenance } } } } }`,
	"list_api_keys":                     `query { currentProject { id tokens { name role expire token_prefix } } }`,
	"create_api_key":                    `mutation($name: String!, $duration: String!, $role: String!) { generateProjectToken(name: $name, duration: $duration, role: $role) { token } }`,
	"delete_api_key":                    `mutation($token: String!, $duration: String!) { deleteProjectToken(token: $token, duration: $duration) { msg } }`,
	"get_auth_settings":                 `query { getProject { authentication_settings { enable_general_auth enable_google_auth default_registration_role } } }`,
	"update_auth_settings":              `mutation($input: UpdateProjectAuthenticationInput!) { updateProjectAuthenticationSettings(input: $input) { authentication_settings { enable_general_auth } } }`,
	"get_storage_settings":              `query { getProject { storage_settings { use_free_cloud_storage endpoint region bucket public_base_url } } }`,
	"update_storage_settings":           `mutation($input: UpdateProjectStorageInput!) { updateProjectStorageSettings(input: $input) { storage_settings { bucket } } }`,
	"list_team_members":                 `query { workspaceMembers { id email } }`,
	"update_team_members":               `mutation($email: String!, $project_ids: [String!]!, $make_admin: Boolean) { inviteWorkspaceMember(email: $email, project_ids: $project_ids, make_admin: $make_admin) }`,
	"list_webhooks":                     `query { listWebHooks { id name model url } }`,
	"create_webhook":                    `mutation($name: String!, $model: String!, $events: [String]!, $url: String, $logic_executions: [String]) { createWebHook(name: $name, model: $model, events: $events, url: $url, logic_executions: $logic_executions) { id name } }`,
	"delete_webhook":                    `mutation($id: String!) { deleteWebHook(id: $id) { msg } }`,
	"list_plugins":                      `query { getProjectPlugins { id title enable } }`,
	"configure_plugin":                  `mutation($id: String!, $enable: Boolean) { upsertPlugin(id: $id, enable: $enable) { id enable } }`,
	"remove_plugin":                     `mutation($id: String!) { removeProjectSpecificPlugin(id: $id) { message } }`,
	"list_functions":                    `query { projectFunctionsInfo { name description trigger_type active_revision_id } }`,
	"upsert_function":                   `mutation($name: String!, $description: String, $source: String, $trigger_type: String, $update: Boolean) { upsertFunctionToProject(name: $name, description: $description, source: $source, trigger_type: $trigger_type, update: $update) { name active_revision_id } }`,
	"delete_function":                   `mutation($function: String!) { deleteFunctionFromProject(function: $function) { name } }`,
	"test_function_draft":               `mutation($name: String!, $source: String, $payload: JSON, $tenant_id: String) { testFunctionDraft(name: $name, source: $source, payload: $payload, tenant_id: $tenant_id) { ok error } }`,
	"deploy_function":                   `mutation($name: String!, $source: String) { deployFunctionToProject(name: $name, source: $source) { function { name active_revision_id } } }`,
	"list_function_revisions":           `query($name: String!, $limit: Int) { listFunctionRevisions(name: $name, limit: $limit) { id revision created_at } }`,
	"list_function_deployments":         `query($name: String!, $limit: Int) { listFunctionDeployments(name: $name, limit: $limit) { id revision_id status created_at } }`,
	"rollback_function":                 `mutation($name: String!, $revision_id: String!) { rollbackFunctionDeployment(name: $name, revision_id: $revision_id) { function { name active_revision_id } } }`,
	"search_system_logs":                `query($text: String, $limit: Int, $project_id: String, $tenant_id: String) { searchSystemLogs(text: $text, limit: $limit, project_id: $project_id, tenant_id: $tenant_id) { total results { id level status error_message } } }`,
	"get_system_log":                    `query($id: String!) { systemLog(id: $id) { id trace_id level status error_message } }`,
	"get_system_trace":                  `query($trace_id: String!) { systemLogTrace(trace_id: $trace_id) { id level status error_message } }`,
	"summarize_system_logs":             `query($project_id: String, $text: String) { systemLogStats(project_id: $project_id, text: $text) { store { total_events } health { engine open error } } }`,
	"get_log_store_health":              `query { systemLogHealth }`,
}

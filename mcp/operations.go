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
	if name == "get_data" || name == "delete_data" || name == "upsert_data" || name == "duplicate_data" {
		if _, ok := out["model"]; !ok {
			if model, ok := out["model_name"].(string); ok {
				out["model"] = model
			}
		}
	}
	return out
}

var operations = map[string]string{
	"list_models":                       `query($model_name: String) { projectModelsInfo(model_name: $model_name) { name } }`,
	"get_model_schema":                  `query($model_name: String) { projectModelsInfo(model_name: $model_name) { name fields { identifier field_type } } }`,
	"get_schema_versioning_status":      `query { schemaVersioningStatus { enabled } }`,
	"get_schema_preview":                `query { schemaPreview { preview } }`,
	"get_effective_schema":              `query { effectiveSchema { preview } }`,
	"get_schema_change_plan":            `query { schemaChangePlan { summary } }`,
	"summarize_schema_draft_for_review": `query { schemaChangePlan { summary } }`,
	"get_model_physical_health":         `query($model_name: String) { modelPhysicalHealth(model_name: $model_name) { model_name } }`,
	"get_project_physical_health":       `query { projectPhysicalHealth { model_name } }`,
	"reconcile_common_models":           `query($model_name: String) { reconcileCommonModels(model_name: $model_name) { name } }`,
	"get_project_context":               `query { currentProject { id name } }`,
	"get_relation_graph":                `query { projectSchemaRelationGraph { models } }`,
	"get_public_graphql_model_map":      `query { projectModelsInfo { name } }`,
	"get_data":                          `query($model: String!, $limit: Int, $page: Int, $search: String) { getModelData(model: $model, limit: $limit, page: $page, search: $search) { count } }`,
	"delete_data":                       `mutation($model: String!, $id: String!) { deleteModelData(model: $model, id: $id) }`,
	"upsert_data":                       `mutation($model: String!, $id: String, $data: JSON) { upsertModelData(model: $model, id: $id, single_page_data: $data) { id } }`,
	"duplicate_data":                    `mutation($model: String!, $id: String!) { duplicateModelData(model: $model, id: $id) { id } }`,
	"list_tenants":                      `query { getTenants { tenants { id name } } }`,
	"search_tenants":                    `query($project_id: String!, $limit: Int, $offset: Int, $q: String, $status: String) { searchTenants(project_id: $project_id, limit: $limit, offset: $offset, q: $q, status: $status) { count } }`,
	"create_tenant":                     `mutation($name: String!, $data: String, $domain: String, $plan_tier: String) { createTenant(name: $name, data: $data, domain: $domain, plan_tier: $plan_tier) { id name } }`,
	"update_tenant":                     `mutation($tenant_id: String!, $name: String, $data: String, $domain: String, $plan_tier: String) { updateTenant(tenant_id: $tenant_id, name: $name, data: $data, domain: $domain, plan_tier: $plan_tier) { id } }`,
	"delete_tenant":                     `mutation($tenant_id: String!) { deleteTenant(tenant_id: $tenant_id) }`,
	"generate_tenant_token":             `mutation($tenant_id: String!, $duration: String!, $role: String) { generateTenantToken(tenant_id: $tenant_id, duration: $duration, role: $role) { tenant_id role } }`,
	"search_tenant_by_domain":           `query($project_id: String!, $domain: String!) { searchTenantsByDomain(project_id: $project_id, domain: $domain) { tenant { id name } } }`,
	"search_app_users":                  `query($project_id: String, $q: String, $tenant_id: String) { searchUsers(project_id: $project_id, q: $q, tenant_id: $tenant_id) { id } }`,
	"list_roles":                        `query { projectRoles { id name } }`,
	"get_project_settings":              `query { currentProject { id name } }`,
	"search_system_logs":                `query($q: String, $limit: Int) { searchSystemLogs(q: $q, limit: $limit) { id } }`,
}

package models

// Extension keys for the SaaS tenant control-plane model (catalog + FK mirror anchor).
// Pro seeds `system_tenant_model`; classification may also set `is_tenant_model`.
const (
	ExtKeyIsSystemTenantModel = "system_tenant_model"
	ExtKeyIsTenantModel       = "is_tenant_model"
)

// ModelIsSaaSTenantControlPlaneModel reports whether m is the reserved SaaS tenant model
// whose lifecycle is managed by explicit system GraphQL (searchTenants/createTenant/…),
// not ordinary dynamic public model CRUD roots.
func ModelIsSaaSTenantControlPlaneModel(m *ModelType) bool {
	if m == nil {
		return false
	}
	if m.Ext != nil {
		if v, ok := m.Ext[ExtKeyIsSystemTenantModel].(bool); ok && v {
			return true
		}
		if v, ok := m.Ext[ExtKeyIsTenantModel].(bool); ok && v {
			return true
		}
	}
	return false
}

package controller

import "github.com/apito-io/engine/models"

// schemaRoleForPublicSchemaBuild returns the role used for permission maps during schema construction.
// When roleAgnostic is true, the role is elevated to admin-equivalent so the built schema is a superset;
// the caller's real role remains on cache.Param for resolver-time enforcement.
func schemaRoleForPublicSchemaBuild(role *models.Role, roleAgnostic bool) *models.Role {
	if role == nil {
		return nil
	}
	if !roleAgnostic {
		return role
	}
	r := *role
	r.IsAdmin = true
	return &r
}

// fingerprintRoleForPreConnectionCache returns the role to include in the pre-connection LRU key.
// When role-agnostic mode is on, the key must not vary by role (nil skips role material in the fingerprint).
func fingerprintRoleForPreConnectionCache(role *models.Role, roleAgnostic bool) *models.Role {
	if roleAgnostic {
		return nil
	}
	return role
}

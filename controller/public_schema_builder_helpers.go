package controller

import (
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/tailor-platform/graphql"
)

// resolveConnectionPermission looks up API permission for a relation target.
// Roles often grant permissions on known_as keys (e.g. "chef") while connection.Model is the base model ("employee").
func resolveConnectionPermission(permissions map[string]*models.APIPermission, conn *models.ConnectionType, role *models.Role) (*models.APIPermission, bool) {
	if conn == nil {
		return nil, false
	}
	keys := []string{conn.Model}
	if conn.KnownAs != "" {
		keys = append([]string{conn.KnownAs}, keys...)
	}
	for _, k := range keys {
		if ap, ok := permissions[k]; ok && ap != nil {
			return ap, true
		}
		if role != nil {
			if ap, ok := utility.LookupAPIPermission(role, k); ok && ap != nil {
				return ap, true
			}
		}
	}
	return nil, false
}

// modelReadAllowed reports whether the role may read documents of modelName.
func modelReadAllowed(permissions map[string]*models.APIPermission, modelName string, role *models.Role) bool {
	if role != nil && role.IsAdmin {
		return true
	}
	if ap, ok := permissions[modelName]; ok && ap != nil {
		return ap.Read != "none"
	}
	if role != nil {
		if ap, ok := utility.LookupAPIPermission(role, modelName); ok && ap != nil {
			return ap.Read != "none"
		}
	}
	return false
}

// connectionModelDisplayName returns KnownAs when set, otherwise the related model name.
func connectionModelDisplayName(conn *models.ConnectionType) string {
	if conn == nil {
		return ""
	}
	if conn.KnownAs != "" {
		return conn.KnownAs
	}
	return conn.Model
}

// cloneGraphQLFields returns a shallow copy of the fields map so mutations (e.g. groupBy keys)
// do not leak into other GraphQL object definitions that share the same backing map.
func cloneGraphQLFields(in graphql.Fields) graphql.Fields {
	if len(in) == 0 {
		return graphql.Fields{}
	}
	out := make(graphql.Fields, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// cloneCommonFieldsByModel deep-copies the per-model field maps for cache replay before
// attachConnectionFields mutates them (adds relation fields).
func cloneCommonFieldsByModel(in map[string]graphql.Fields) map[string]graphql.Fields {
	if len(in) == 0 {
		return map[string]graphql.Fields{}
	}
	out := make(map[string]graphql.Fields, len(in))
	for modelName, fields := range in {
		out[modelName] = cloneGraphQLFields(fields)
	}
	return out
}

// cloneFieldConfigArgumentMap copies top-level mutation arg maps so cache replay can add payload keys safely.
func cloneFieldConfigArgumentMap(in map[string]graphql.FieldConfigArgument) map[string]graphql.FieldConfigArgument {
	if len(in) == 0 {
		return map[string]graphql.FieldConfigArgument{}
	}
	out := make(map[string]graphql.FieldConfigArgument, len(in))
	for k, v := range in {
		inner := make(graphql.FieldConfigArgument, len(v))
		for ik, iv := range v {
			inner[ik] = iv
		}
		out[k] = inner
	}
	return out
}

func cloneStringToInputObjectFieldMap(in map[string]graphql.InputObjectConfigFieldMap) map[string]graphql.InputObjectConfigFieldMap {
	if len(in) == 0 {
		return map[string]graphql.InputObjectConfigFieldMap{}
	}
	out := make(map[string]graphql.InputObjectConfigFieldMap, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func cloneStringToInputObjectPtr(in map[string]*graphql.InputObject) map[string]*graphql.InputObject {
	if len(in) == 0 {
		return map[string]*graphql.InputObject{}
	}
	out := make(map[string]*graphql.InputObject, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

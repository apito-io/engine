package utility

import (
	"errors"

	"github.com/apito-io/engine/models"
)

type CRUDPermissions struct {
	Create bool
	Read   bool
	Update bool
	Delete bool
}

// LookupAPIPermission returns role permissions for a model id (canonical snake or legacy camel key).
func LookupAPIPermission(role *models.Role, modelName string) (*models.APIPermission, bool) {
	if role == nil || role.APIPermissions == nil {
		return nil, false
	}
	if ap, ok := role.APIPermissions[modelName]; ok {
		return ap, true
	}
	if ap, ok := role.APIPermissions[CamelFromAny(modelName)]; ok {
		return ap, true
	}
	return nil, false
}

func BuildCRUDPermissions(modelName string, role *models.Role) (*models.APIPermission, error) {

	if role == nil {
		return nil, errors.New("Role Cant be Nil")
	}

	if role.IsAdmin || role.SystemGenerated {
		return &models.APIPermission{
			Read:   "all",
			Create: "all",
			Update: "all",
			Delete: "all",
		}, nil
	}

	if ap, ok := LookupAPIPermission(role, modelName); ok {
		return ap, nil
	}
	// if not found then assign none for all thing
	return &models.APIPermission{
		Read:   "none",
		Create: "none",
		Update: "none",
		Delete: "none",
	}, nil
}

func ValidatePermissions(vv map[string]interface{}) (*models.APIPermission, error) {
	var p = &models.APIPermission{}
	if val, ok := ValidateScope(vv["read"]); ok && val != nil {
		p.Read = *val
	} else {
		return nil, errors.New("invalid Read Permissions")
	}
	if val, ok := ValidateScope(vv["create"]); ok && val != nil {
		p.Create = *val
	} else {
		return nil, errors.New("invalid Create Permissions")
	}
	if val, ok := ValidateScope(vv["update"]); ok && val != nil {
		p.Update = *val
	} else {
		return nil, errors.New("invalid Update Permissions")
	}
	if val, ok := ValidateScope(vv["delete"]); ok && val != nil {
		p.Delete = *val
	} else {
		return nil, errors.New("invalid Delete Permissions")
	}
	return p, nil
}

// ValidateScope accepts permission scope strings stored in roles (see public mutations / schema builder).
// nil or non-string values are invalid; do not type-assert map values to string at the call site.
func ValidateScope(p interface{}) (*string, bool) {
	if p == nil {
		none := "none"
		return &none, true
	}
	s, ok := p.(string)
	if !ok || s == "" {
		return nil, false
	}
	switch s {
	case "none", "all", "custom_logic", "own", "auth":
		out := s
		return &out, true
	default:
		return nil, false
	}
}

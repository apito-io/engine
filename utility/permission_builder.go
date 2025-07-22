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

	if ap, ok := role.APIPermissions[modelName]; ok {
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
	if val, ok := ValidateScope(vv["read"].(string)); ok {
		p.Read = *val
	} else {
		return nil, errors.New("invalid Read Permissions")
	}
	if val, ok := ValidateScope(vv["create"].(string)); ok {
		p.Create = *val
	} else {
		return nil, errors.New("invalid Create Permissions")
	}
	if val, ok := ValidateScope(vv["update"].(string)); ok {
		p.Update = *val
	} else {
		return nil, errors.New("invalid Update Permissions")
	}
	if val, ok := ValidateScope(vv["delete"].(string)); ok {
		p.Delete = *val
	} else {
		return nil, errors.New("invalid Delete Permissions")
	}
	return p, nil
}

func ValidateScope(p string) (*string, bool) {
	if p == "none" || p == "all" || p == "custom_logic" {
		return &p, true
	}
	return nil, false
}

package resolver

import (
	"errors"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// enforceRoleAgnosticModelRead rejects read access when the public schema was built in
// role-agnostic (superset) mode and the real role has no read permission for the model.
func (s *GraphQLServer) enforceRoleAgnosticModelRead(modelName string, role *models.Role) error {
	if s.Cfg == nil || !s.Cfg.RoleAgnosticSchemaCache || role == nil {
		return nil
	}
	perm, err := utility.BuildCRUDPermissions(modelName, role)
	if err != nil {
		return err
	}
	if perm != nil && perm.Read == "none" {
		return errors.New("permission denied: read not allowed for this model")
	}
	return nil
}

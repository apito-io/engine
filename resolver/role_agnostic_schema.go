package resolver

import (
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// enforceRoleAgnosticModelRead rejects read access when the role has no read permission
// for the model (none/auth/own). Applied on every public query path — not only when
// RoleAgnosticSchemaCache is enabled — so schema-superset exposure cannot bypass ACL.
func (s *GraphQLServer) enforceRoleAgnosticModelRead(modelName string, role *models.Role) error {
	return utility.AuthorizeModelRead(role, modelName)
}

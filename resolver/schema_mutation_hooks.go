package resolver

import (
	"errors"
	"fmt"

	"github.com/apito-io/engine/models"
	"github.com/tailor-platform/graphql"
)

func (s *GraphQLServer) schemaMutationHook() models.SchemaMutationHook {
	if s == nil || s.Cfg == nil || s.Cfg.SchemaMutationHook == nil {
		return nil
	}
	return s.Cfg.SchemaMutationHook
}

func (s *GraphQLServer) schemaVersioningActive() bool {
	if s == nil || s.Cfg == nil {
		return false
	}
	return s.Cfg.SchemaVersioningEnabled && !s.Cfg.SchemaVersioningBypass
}

func (s *GraphQLServer) tryStageSchemaMutation(
	cache *models.ApplicationCache,
	project *models.Project,
	operationType string,
	args map[string]interface{},
	defaultFn func() (interface{}, error),
) (interface{}, error) {
	if s.schemaVersioningActive() && s.schemaMutationHook() == nil {
		return nil, fmt.Errorf("schema versioning is enabled but SchemaMutationHook is not registered")
	}

	hook := s.schemaMutationHook()
	if hook == nil {
		return defaultFn()
	}
	userID, roleID := "", ""
	if cache != nil && cache.Param != nil {
		userID = cache.Param.UserID
		if cache.Param.Role != nil {
			roleID = cache.Param.Role.ID
		}
	}
	req := &models.SchemaMutationRequest{
		OperationType: operationType,
		Project:       project,
		Args:          args,
		UserID:        userID,
		Role:          roleID,
	}
	res, handled, err := hook(cache.Ctx, req)
	if err != nil {
		return nil, err
	}
	if !handled {
		if s.schemaVersioningActive() {
			return nil, errors.New("schema versioning hook declined to stage mutation")
		}
		return defaultFn()
	}
	if res != nil && res.Response != nil {
		return res.Response, nil
	}
	return res, nil
}

func graphqlArgsMap(p graphql.ResolveParams) map[string]interface{} {
	out := make(map[string]interface{}, len(p.Args))
	for k, v := range p.Args {
		out[k] = v
	}
	return out
}

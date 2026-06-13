package resolver

import (
	"github.com/tailor-platform/graphql"
)

// ProjectUserGraphQLHook runs before the open-core default for a project end-user GraphQL operation.
// Return handled=true to skip the default implementation.
type ProjectUserGraphQLHook func(s *GraphQLServer, p graphql.ResolveParams) (result interface{}, handled bool, err error)

// ProjectUserItemFieldsHook lets the host extend the project end-user GraphQL object.
// Open-core does not define or interpret host field names.
type ProjectUserItemFieldsHook func(userItem *graphql.Object)

// ProjectUserGraphQLOperationFieldHook lets the host extend a named project end-user GraphQL
// operation field (e.g. add Args). operation is the root field name: createUser, searchUsers, etc.
// Open-core does not define or interpret host argument names.
type ProjectUserGraphQLOperationFieldHook func(operation string, field *graphql.Field)

// ProjectUserGraphQLHooks groups optional host-provided overrides for project end-user GraphQL ops.
type ProjectUserGraphQLHooks struct {
	SearchUsers       ProjectUserGraphQLHook
	LoginUser         ProjectUserGraphQLHook
	CreateUser        ProjectUserGraphQLHook
	UpdateUser        ProjectUserGraphQLHook
	DeleteUser        ProjectUserGraphQLHook
	ResetUserPassword ProjectUserGraphQLHook
	GoogleOAuthState  ProjectUserGraphQLHook
}

func (s *GraphQLServer) projectUserHooks() *ProjectUserGraphQLHooks {
	if s == nil || s.Cfg == nil || s.Cfg.ProjectUserGraphQLHooks == nil {
		return nil
	}
	h, ok := s.Cfg.ProjectUserGraphQLHooks.(*ProjectUserGraphQLHooks)
	if !ok {
		return nil
	}
	return h
}

func (s *GraphQLServer) tryProjectUserHook(hook ProjectUserGraphQLHook, p graphql.ResolveParams) (interface{}, bool, error) {
	if hook == nil {
		return nil, false, nil
	}
	return hook(s, p)
}

// runProjectUserHook runs a host hook; returns handled=true to skip open-core default, or any hook error.
func (s *GraphQLServer) runProjectUserHook(hook ProjectUserGraphQLHook, p graphql.ResolveParams) (result interface{}, stop bool, err error) {
	res, handled, err := s.tryProjectUserHook(hook, p)
	if handled || err != nil {
		return res, true, err
	}
	return res, false, nil
}

func (s *GraphQLServer) applyProjectUserItemFieldsHook(userItem *graphql.Object) {
	if s == nil || s.Cfg == nil || s.Cfg.ProjectUserItemFieldsHook == nil || userItem == nil {
		return
	}
	hook, ok := s.Cfg.ProjectUserItemFieldsHook.(ProjectUserItemFieldsHook)
	if !ok || hook == nil {
		return
	}
	hook(userItem)
}

func (s *GraphQLServer) applyProjectUserGraphQLOperationFieldHook(operation string, field *graphql.Field) {
	if s == nil || s.Cfg == nil || s.Cfg.ProjectUserGraphQLOperationFieldHook == nil || field == nil {
		return
	}
	hook, ok := s.Cfg.ProjectUserGraphQLOperationFieldHook.(ProjectUserGraphQLOperationFieldHook)
	if !ok || hook == nil {
		return
	}
	hook(operation, field)
}

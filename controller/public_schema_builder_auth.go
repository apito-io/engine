package controller

import (
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
)

// publicAuthQueryNameSet returns the first-class public auth query roots
// (loginUser, myEffectivePermissions, host-extended myTenant, …).
func publicAuthQueryNameSet(s *resolver.GraphQLServer) map[string]struct{} {
	out := make(map[string]struct{})
	if s == nil {
		return out
	}
	for name, field := range s.PublicAuthQueryFields() {
		if name == "" || field == nil {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}

func isIntrospectionRootField(name string) bool {
	switch name {
	case "__typename", "__schema", "__type":
		return true
	default:
		return false
	}
}

// incomingRequestIsPublicAuthOnly is true when every non-introspection root
// field is a PublicAuthQueryFields entry. Those queries are not model/function
// operations, so schema build must not require filteredModels/Functions.
func incomingRequestIsPublicAuthOnly(cache *models.ApplicationCache, authNames map[string]struct{}) bool {
	if cache == nil || len(cache.IncomingRequest) == 0 || len(authNames) == 0 {
		return false
	}
	sawAuthRoot := false
	for _, req := range cache.IncomingRequest {
		if req == nil {
			continue
		}
		if len(req.RootFields) == 0 {
			return false
		}
		for _, name := range req.RootFields {
			if isIntrospectionRootField(name) {
				continue
			}
			if _, ok := authNames[name]; !ok {
				return false
			}
			sawAuthRoot = true
		}
	}
	return sawAuthRoot
}

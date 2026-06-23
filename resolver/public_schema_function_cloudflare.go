//go:build cloudflare

package resolver

import (
	"context"
	"errors"

	"github.com/apito-io/engine/models"
	"github.com/tailor-platform/graphql"
)

func (s *GraphQLServer) HandleApitoFunction(_ context.Context, _ *models.ApplicationCache, _ string, _ map[string]interface{}) (interface{}, *models.ApitoFunction, error) {
	return nil, nil, errors.New("functions not available in Workers build")
}

func (s *GraphQLServer) ApitoFunctionResolverFn(_ graphql.ResolveParams) (interface{}, error) {
	return nil, errors.New("functions not available in Workers build")
}

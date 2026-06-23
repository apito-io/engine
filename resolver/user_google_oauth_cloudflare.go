//go:build cloudflare

package resolver

import (
	"context"
	"errors"

	"github.com/apito-io/engine/models"
	"github.com/tailor-platform/graphql"
)

func errGoogleLoginDeferred() error {
	return errors.New("google login is not available on Cloudflare Workers v1")
}

// GoogleOAuthStateResolverFn is deferred on Workers v1.
func (s *GraphQLServer) GoogleOAuthStateResolverFn(_ graphql.ResolveParams) (interface{}, error) {
	return nil, errGoogleLoginDeferred()
}

func (s *GraphQLServer) loginUserGoogleOAuth(
	_ context.Context,
	_ *models.ApplicationCache,
	_ *ProjectUserService,
	_ *models.Project,
	_ map[string]interface{},
) (interface{}, error) {
	return nil, errGoogleLoginDeferred()
}

func (s *GraphQLServer) loginUserGoogleIDToken(
	_ context.Context,
	_ *models.ApplicationCache,
	_ *ProjectUserService,
	_ *models.Project,
	_ map[string]interface{},
) (interface{}, error) {
	return nil, errGoogleLoginDeferred()
}

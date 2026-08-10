package resolver

import "github.com/tailor-platform/graphql"

// ExtendPublicAuthQueryFieldsHook lets the host append fields to the public auth query set.
type ExtendPublicAuthQueryFieldsHook func(fields graphql.Fields)

// PublicAuthQueryFields returns static auth query fields appended to the public GraphQL schema.
func (s *GraphQLServer) PublicAuthQueryFields() graphql.Fields {
	userItem := s.ensureUserItemGraphQLObject()
	loginUserPayload := graphql.NewObject(graphql.ObjectConfig{
		Name: "PublicLoginUserPayload",
		Fields: graphql.Fields{
			"token": &graphql.Field{Type: graphql.String},
			"user":  &graphql.Field{Type: userItem},
		},
	})
	oauthStatePayload := graphql.NewObject(graphql.ObjectConfig{
		Name: "PublicOAuthStatePayload",
		Fields: graphql.Fields{
			"state": &graphql.Field{Type: graphql.String},
		},
	})
	googleOAuthStatePayload := graphql.NewObject(graphql.ObjectConfig{
		Name: "PublicGoogleOAuthStatePayload",
		Fields: graphql.Fields{
			"state": &graphql.Field{Type: graphql.String},
		},
	})
	loginUserField := &graphql.Field{
		Type: loginUserPayload,
		Args: graphql.FieldConfigArgument{
			"auth_method": &graphql.ArgumentConfig{Type: graphql.String},
			"project_id":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"phone":       &graphql.ArgumentConfig{Type: graphql.String},
			"email":       &graphql.ArgumentConfig{Type: graphql.String},
			"password":    &graphql.ArgumentConfig{Type: graphql.String},
			"code":        &graphql.ArgumentConfig{Type: graphql.String},
			"state":       &graphql.ArgumentConfig{Type: graphql.String},
			"id_token":    &graphql.ArgumentConfig{Type: graphql.String},
			"signup":      &graphql.ArgumentConfig{Type: graphql.Boolean},
		},
		Resolve: s.LoginUserResolverFn,
	}
	s.applyProjectUserGraphQLOperationFieldHook("loginUser", loginUserField)

	fields := graphql.Fields{
		"loginUser": loginUserField,
		"oauthState": &graphql.Field{
			Type: oauthStatePayload,
			Args: graphql.FieldConfigArgument{
				"provider":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"project_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: s.OAuthStateResolverFn,
		},
		"googleOAuthState": &graphql.Field{
			Type: googleOAuthStatePayload,
			Args: graphql.FieldConfigArgument{
				"project_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: s.GoogleOAuthStateResolverFn,
		},
		"myEffectivePermissions": &graphql.Field{
			Type:        effectivePermissionsPayloadType(),
			Description: "Effective app-user permissions after tenant plan ceiling (scopes, quotas, grace).",
			Resolve:     s.MyEffectivePermissionsResolverFn,
		},
	}
	s.applyExtendPublicAuthQueryFieldsHook(fields)
	return fields
}

func (s *GraphQLServer) applyExtendPublicAuthQueryFieldsHook(fields graphql.Fields) {
	if s == nil || s.Cfg == nil || s.Cfg.ExtendPublicAuthQueryFieldsHook == nil || fields == nil {
		return
	}
	hook, ok := s.Cfg.ExtendPublicAuthQueryFieldsHook.(ExtendPublicAuthQueryFieldsHook)
	if !ok || hook == nil {
		return
	}
	hook(fields)
}

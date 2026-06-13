package resolver

import "github.com/tailor-platform/graphql"

// PublicAuthQueryFields returns static auth query fields appended to the public GraphQL schema.
func (s *GraphQLServer) PublicAuthQueryFields() graphql.Fields {
	if UserItemGraphQLObject == nil {
		s.RegisterUserSchema()
	}
	userItem := UserItemGraphQLObject
	loginUserPayload := graphql.NewObject(graphql.ObjectConfig{
		Name: "PublicLoginUserPayload",
		Fields: graphql.Fields{
			"token": &graphql.Field{Type: graphql.String},
			"user":  &graphql.Field{Type: userItem},
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
		},
		Resolve: s.LoginUserResolverFn,
	}
	s.applyProjectUserGraphQLOperationFieldHook("loginUser", loginUserField)

	return graphql.Fields{
		"loginUser": loginUserField,
		"googleOAuthState": &graphql.Field{
			Type: googleOAuthStatePayload,
			Args: graphql.FieldConfigArgument{
				"project_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: s.GoogleOAuthStateResolverFn,
		},
	}
}

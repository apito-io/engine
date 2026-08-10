package resolver

import (
	"github.com/tailor-platform/graphql"
)

// UserItemGraphQLObject is the open-core UserItem GraphQL type.
var UserItemGraphQLObject *graphql.Object

// ensureUserItemGraphQLObject builds the shared UserItem type without touching
// system schema channels. Safe for public auth schema assembly and tests.
func (s *GraphQLServer) ensureUserItemGraphQLObject() *graphql.Object {
	if UserItemGraphQLObject != nil {
		return UserItemGraphQLObject
	}
	userItem := graphql.NewObject(graphql.ObjectConfig{
		Name: "UserItem",
		Fields: graphql.Fields{
			"id":         &graphql.Field{Type: graphql.String},
			"email":      &graphql.Field{Type: graphql.String},
			"username":   &graphql.Field{Type: graphql.String},
			"phone":      &graphql.Field{Type: graphql.String},
			"role":       &graphql.Field{Type: graphql.String},
			"provider":   &graphql.Field{Type: graphql.String},
			"status":     &graphql.Field{Type: graphql.String},
			"created_at": &graphql.Field{Type: graphql.String},
			"updated_at": &graphql.Field{Type: graphql.String},
		},
	})
	s.applyProjectUserItemFieldsHook(userItem)
	UserItemGraphQLObject = userItem
	return userItem
}

// RegisterUserSchema registers project end-user operations on the system GraphQL schema.
func (s *GraphQLServer) RegisterUserSchema() {
	userItem := s.ensureUserItemGraphQLObject()

	loginUserPayload := graphql.NewObject(graphql.ObjectConfig{
		Name: "LoginUserPayload",
		Fields: graphql.Fields{
			"token": &graphql.Field{Type: graphql.String},
			"user":  &graphql.Field{Type: userItem},
		},
	})

	searchUsersPayload := graphql.NewObject(graphql.ObjectConfig{
		Name: "SearchUsersPayload",
		Fields: graphql.Fields{
			"users": &graphql.Field{Type: graphql.NewList(userItem)},
			"count": &graphql.Field{Type: graphql.Int},
		},
	})

	oauthStatePayload := graphql.NewObject(graphql.ObjectConfig{
		Name: "OAuthStatePayload",
		Fields: graphql.Fields{
			"state": &graphql.Field{Type: graphql.String},
		},
	})
	googleOAuthStatePayload := graphql.NewObject(graphql.ObjectConfig{
		Name: "GoogleOAuthStatePayload",
		Fields: graphql.Fields{
			"state": &graphql.Field{Type: graphql.String},
		},
	})

	searchUsersField := &graphql.Field{
		Type: searchUsersPayload,
		Args: graphql.FieldConfigArgument{
			"project_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"limit":      &graphql.ArgumentConfig{Type: graphql.Int},
			"offset":     &graphql.ArgumentConfig{Type: graphql.Int},
			"q":          &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: s.SearchUsersResolverFn,
	}
	s.applyProjectUserGraphQLOperationFieldHook("searchUsers", searchUsersField)

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
			// When false, Google/OAuth must find an existing account (no auto-provision / create).
			// Omit or true: keep signup auto-provision when tenant_id is empty.
			"signup": &graphql.ArgumentConfig{Type: graphql.Boolean},
		},
		Resolve: s.LoginUserResolverFn,
	}
	s.applyProjectUserGraphQLOperationFieldHook("loginUser", loginUserField)

	if s.SystemQueriesChan != nil {
		s.SystemQueriesChan <- &graphql.Fields{
			"searchUsers": searchUsersField,
			"loginUser":   loginUserField,
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
		}
	}

	createUserField := &graphql.Field{
		Type: userItem,
		Args: graphql.FieldConfigArgument{
			"project_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"password":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"role":       &graphql.ArgumentConfig{Type: graphql.String},
			"email":      &graphql.ArgumentConfig{Type: graphql.String},
			"phone":      &graphql.ArgumentConfig{Type: graphql.String},
			"username":   &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: s.CreateUserResolverFn,
	}
	s.applyProjectUserGraphQLOperationFieldHook("createUser", createUserField)

	// Public self-signup: same fields as createUser minus role (forced to
	// default_registration_role). Callable with a non-admin project API key.
	registerUserField := &graphql.Field{
		Type: userItem,
		Args: graphql.FieldConfigArgument{
			"project_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"password":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"email":      &graphql.ArgumentConfig{Type: graphql.String},
			"phone":      &graphql.ArgumentConfig{Type: graphql.String},
			"username":   &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: s.RegisterUserResolverFn,
	}
	s.applyProjectUserGraphQLOperationFieldHook("registerUser", registerUserField)

	updateUserField := &graphql.Field{
		Type: userItem,
		Args: graphql.FieldConfigArgument{
			"user_id":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			"email":    &graphql.ArgumentConfig{Type: graphql.String},
			"phone":    &graphql.ArgumentConfig{Type: graphql.String},
			"role":     &graphql.ArgumentConfig{Type: graphql.String},
			"username": &graphql.ArgumentConfig{Type: graphql.String},
		},
		Resolve: s.UpdateUserResolverFn,
	}
	s.applyProjectUserGraphQLOperationFieldHook("updateUser", updateUserField)

	if s.SystemMutationsChan != nil {
		s.SystemMutationsChan <- &graphql.Fields{
			"createUser":   createUserField,
			"registerUser": registerUserField,
			"updateUser":   updateUserField,
			"deleteUser": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"user_id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: s.DeleteUserResolverFn,
			},
			"resetUserPassword": &graphql.Field{
				Type: graphql.Boolean,
				Args: graphql.FieldConfigArgument{
					"user_id":  &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"password": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: s.ResetUserPasswordResolverFn,
			},
		}
	}
}

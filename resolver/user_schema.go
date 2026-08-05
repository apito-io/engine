package resolver

import (
	"github.com/tailor-platform/graphql"
)

// UserItemGraphQLObject is the open-core UserItem GraphQL type.
var UserItemGraphQLObject *graphql.Object

// RegisterUserSchema registers project end-user operations on the system GraphQL schema.
func (s *GraphQLServer) RegisterUserSchema() {
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
		},
		Resolve: s.LoginUserResolverFn,
	}
	s.applyProjectUserGraphQLOperationFieldHook("loginUser", loginUserField)

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

	s.SystemMutationsChan <- &graphql.Fields{
		"createUser": createUserField,
		"updateUser": updateUserField,
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

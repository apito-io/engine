package schemas

import (
	"context"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/scaler"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
	"github.com/tailor-inc/graphql/gqlerrors"
	"github.com/teivah/onecontext"
)

func SystemSchema(ctx context.Context, graphqlRequest *models.GraphQLIncomingRequest, server *resolver.GraphQLServer, echo echo.Context) (*graphql.Result, error) {

	role := echo.Get("role")
	if role == "demo" {
		return &graphql.Result{Errors: []gqlerrors.FormattedError{
			{
				Message: "You Cant Change Anything in a Demo Project",
			},
		}}, nil
	}
	
	/*	subscriptions := graphql.Fields{
		"notifySystem": &graphql.Field{
			Name: "NotifySystemType",
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "NotifySystemResponse",
				Fields: graphql.Fields{
					"message":  &graphql.Field{Type: graphql.String},
					"type":     &graphql.Field{Type: graphql.String},
					"duration": &graphql.Field{Type: graphql.Int},
					"final":    &graphql.Field{Type: graphql.String},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"type": &graphql.ArgumentConfig{
					Type: graphql.String,
				},
			},
			Resolve: server.EventSubscription,
		},
	}*/

	schema, err := graphql.NewSchema(
		graphql.SchemaConfig{
			Query: graphql.NewObject(graphql.ObjectConfig{
				Name:   "QueryType",
				Fields: server.SystemQueries,
			}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{
				Name:   "MutationQuery",
				Fields: server.SystemMutations,
			}),
			Types: []graphql.Type{
				scaler.ScalarJSON,
				scaler.ScalarJSONArray,
				// Add built-in scalar types to ensure they are registered
				graphql.String,
				graphql.Int,
				graphql.Float,
				graphql.Boolean,
				graphql.ID,
			},
			/*
				Subscription: graphql.NewObject(graphql.ObjectConfig{
					Name:   "Subscription",
					Fields: subscriptions,
				}),
			*/
		})
	if err != nil {
		return nil, err
	}

	routerCtx := context.WithValue(context.Background(), "router", echo)
	//projectID := context.WithValue(context.Background(), "project_id", cache.Project.ID)
	//tempTenantID := context.WithValue(context.Background(), "tenant_id", cache.Param.TenantID)
	ctx, closeContext := onecontext.Merge(ctx, routerCtx)
	defer closeContext()

	return graphql.Do(graphql.Params{
		Context:        ctx,
		Schema:         schema,
		RequestString:  graphqlRequest.Query,
		VariableValues: graphqlRequest.Variables,
		OperationName:  graphqlRequest.OperationName,
	}), nil
}

func SystemSubscriptionSchema(ctx context.Context, server *resolver.GraphQLServer) (*graphql.Schema, error) {

	query := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"healthcheck": &graphql.Field{
				Type: graphql.String,
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					return "check!", nil
				},
			},
		},
	})
	subscriptions := graphql.NewObject(graphql.ObjectConfig{
		Name: "Subscription",
		Fields: graphql.Fields{
			"notifySystem": &graphql.Field{
				Name: "NotifySystemType",
				Type: graphql.NewObject(graphql.ObjectConfig{
					Name: "NotifySystemResponse",
					Fields: graphql.Fields{
						"message":  &graphql.Field{Type: graphql.String},
						"type":     &graphql.Field{Type: graphql.String},
						"duration": &graphql.Field{Type: graphql.Int},
						"final":    &graphql.Field{Type: graphql.String},
					},
				}),
				Args: graphql.FieldConfigArgument{
					"type": &graphql.ArgumentConfig{
						Type: graphql.String,
					},
				},
				Resolve: func(p graphql.ResolveParams) (interface{}, error) {
					// values sent on channel, that were returned from `Subscribe`, will be available here as
					// `p.Source`
					return p.Source, nil
				},
				Subscribe: server.EventSubscription,
			},
		},
	})

	schema, err := graphql.NewSchema(
		graphql.SchemaConfig{
			Query: query,
			//Mutation:     mutation,
			Subscription: subscriptions,
		})
	if err != nil {
		return nil, err
	}

	return &schema, nil
}

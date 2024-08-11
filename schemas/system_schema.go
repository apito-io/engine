package schemas

import (
	"context"

	"github.com/apito-io/buffers/shared"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/resolver"
	"github.com/apito-io/engine/utility"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
	"github.com/teivah/onecontext"
)

func SystemSchema(ctx context.Context, cache *shared.ApplicationCache, graphqlRequest *models.GraphQLIncomingRequest, server *resolver.GraphQLServer, echo echo.Context) (*graphql.Result, error) {

	incomingRequest, err := utility.ExtractGraphQLOperationName(graphqlRequest.Query, cache.Project.Schema, true)
	if err != nil {
		return nil, err
	}

	queries := make(graphql.Fields)
	mutations := make(graphql.Fields)

	if incomingRequest == nil {
		queries = server.SystemQueries
		mutations = server.SystemMutations
	} else {
		for _, req := range incomingRequest {
			switch req.OperationType {
			case "query":
				for _, query := range req.FilteredModels {
					if val, ok := server.SystemQueries[query.Name]; ok {
						queries[query.Name] = val
						break
					}
				}
			case "mutation":
				for _, query := range req.FilteredModels {
					if val, ok := server.SystemMutations[query.Name]; ok {
						mutations[query.Name] = val
						break
					}
				}
			}
		}
	}

	// default query
	queries["currentProject"] = &graphql.Field{
		Name:    "GetCurrentProject",
		Type:    server.PrivateSchemaObjects.ProjectDetailsObject,
		Resolve: server.GetCurrentProjectResolverFn,
	}

	var _query *graphql.Object
	var _mutation *graphql.Object
	if len(queries) > 0 {
		_query = graphql.NewObject(graphql.ObjectConfig{
			Name:   "QueryType",
			Fields: queries,
		})
	}

	if len(mutations) > 0 {
		_mutation = graphql.NewObject(graphql.ObjectConfig{
			Name:   "MutationQuery",
			Fields: mutations,
		})
	}

	schema, err := graphql.NewSchema(
		graphql.SchemaConfig{
			Query:    _query,
			Mutation: _mutation,
		})
	if err != nil {
		return nil, err
	}

	routerCtx := context.WithValue(context.Background(), "router", echo)

	ctx, closeContext := onecontext.Merge(routerCtx)
	defer closeContext()

	return graphql.Do(graphql.Params{
		Context:        ctx,
		Schema:         schema,
		RequestString:  graphqlRequest.Query,
		VariableValues: graphqlRequest.Variables,
		OperationName:  graphqlRequest.OperationName,
	}), nil
}

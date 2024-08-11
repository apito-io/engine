package resolver

import (
	"errors"
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	ae "github.com/apito-io/engine/err"
	"github.com/jinzhu/inflection"
	"github.com/tailor-inc/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"strings"
)

func (s *GraphQLServer) RootResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v = p.Context.Value
		//router           = v("router").(echo.Context)
		cache            = v("cache").(*shared.ApplicationCache)
		rootSelectionSet = v("selectionSet").(ast.SelectionSet)
		ctx              = p.Context
	)

	model := inflection.Singular(p.Info.FieldName)

	var selectionSet *ast.SelectionSet
	for _, s := range rootSelectionSet {
		if val := s.(*ast.Field); inflection.Singular(val.Name) == model {
			selectionSet = &val.SelectionSet
			break
		}
	}

	/*cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}*/

	model = inflection.Singular(model)

	var modelType *protobuff.ModelType
	for _, field := range cache.Project.Schema.Models {
		if field.Name == model {
			modelType = field
			break
		}
	}

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	param := s.NewParam(cache.Param)

	param.Model = modelType
	param.ResolveParams = &p

	param.QuerySelectionSets = selectionSet

	if uid, id := p.Args["_id"].(string); id {
		param.DocumentId = uid
		doc, err := s.GraphQLExecutor.GetProjectDriver(ctx).GetSingleProjectDocument(p.Context, *param)
		if err != nil {
			return nil, err
		}
		return doc, nil
	}

	if modelType.SinglePage {
		param.DocumentId = modelType.SinglePageUuid
		doc, err := s.GraphQLExecutor.GetProjectDriver(ctx).GetSingleProjectDocument(p.Context, *param)
		if err != nil {
			return nil, err
		}
		return doc, nil
	}

	return s.GraphQLExecutor.GetProjectDriver(ctx).QueryMultiDocumentOfProject(p.Context, *param)
}

func (s *GraphQLServer) CountResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v = p.Context.Value
		//router = v("router").(echo.Context)
		cache = v("cache").(*shared.ApplicationCache)
		ctx   = p.Context
	)

	/*cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}*/

	model := inflection.Singular(strings.TrimSuffix(p.Info.FieldName, "Connection"))

	var modelType *protobuff.ModelType
	for _, field := range cache.Project.Schema.Models {
		if field.Name == model {
			modelType = field
		}
	}

	if modelType == nil {
		return "", errors.New("Connection >  Model Type Not Found")
	}

	param := s.NewParam(cache.Param)
	param.Model = modelType
	param.ResolveParams = &p

	result, err := s.GraphQLExecutor.GetProjectDriver(ctx).CountDocOfProject(p.Context, param)
	if err != nil {
		return nil, err
	}

	return result, nil
}

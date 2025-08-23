package resolver

import (
	"errors"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/tailor-inc/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

func (s *GraphQLServer) MultiResourceResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v = p.Context.Value
		//router           = v("router").(echo.Context)
		cache            = v("cache").(*models.ApplicationCache)
		rootSelectionSet = v("selectionSet").(ast.SelectionSet)
		ctx              = p.Context
	)

	model := utility.SingularResourceName(p.Info.FieldName)

	var selectionSet *ast.SelectionSet
	for _, s := range rootSelectionSet {
		if val := s.(*ast.Field); utility.SingularResourceName(val.Name) == model {
			selectionSet = &val.SelectionSet
			break
		}
	}

	/*cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}*/

	model = utility.SingularResourceName(model)

	var modelType *models.ModelType
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

	driver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}

	return driver.QueryMultiDocumentOfProject(p.Context, param)
}

func (s *GraphQLServer) SingleResourceResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v = p.Context.Value
		//router           = v("router").(echo.Context)
		cache            = v("cache").(*models.ApplicationCache)
		rootSelectionSet = v("selectionSet").(ast.SelectionSet)
		ctx              = p.Context
	)

	model := utility.SingularResourceName(p.Info.FieldName)

	var selectionSet *ast.SelectionSet
	for _, s := range rootSelectionSet {
		if val := s.(*ast.Field); utility.SingularResourceName(val.Name) == model {
			selectionSet = &val.SelectionSet
			break
		}
	}

	/*cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}*/

	model = utility.SingularResourceName(model)

	var modelType *models.ModelType
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

	if param.TenantID == "" && len(p.Args) == 0 {
		return nil, errors.New("ID is required")
	}

	if modelType.IsTenantModel && param.TenantID != "" {
		param.DocumentID = param.TenantID
	} else {
		if uid, ok := p.Args["_id"].(string); ok && uid != "" {
			param.DocumentID = uid
		} else if modelType.SinglePage {
			param.DocumentID = modelType.SinglePageUUID
		}
	}

	driver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}

	doc, err := driver.GetSingleProjectDocument(p.Context, param)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *GraphQLServer) CountResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v = p.Context.Value
		//router = v("router").(echo.Context)
		cache = v("cache").(*models.ApplicationCache)
		ctx   = p.Context
	)

	/*cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}*/

	pp := p.Source.(graphql.ResolveParams)

	model := utility.SingularResourceName(pp.Info.FieldName)

	var modelType *models.ModelType
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
	param.ResolveParams = &pp
	param.OnlyReturnCount = true

	driver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}

	result, err := driver.CountDocOfProject(p.Context, param)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *GraphQLServer) AggregateResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v = p.Context.Value
		//router = v("router").(echo.Context)
		cache = v("cache").(*models.ApplicationCache)
		rootSelectionSet = v("selectionSet").(ast.SelectionSet)
		ctx   = p.Context
	)

	/*cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}*/

	pp := p.Source.(graphql.ResolveParams)

	model := utility.SingularResourceName(pp.Info.FieldName)

	var selectionSet *ast.SelectionSet
	for _, s := range rootSelectionSet {
		if val := s.(*ast.Field); utility.SingularResourceName(val.Name) == model {
			selectionSet = &val.SelectionSet
			break
		}
	}

	var modelType *models.ModelType
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
	param.ResolveParams = &pp
	param.IsAggregateQuery = true

	// this is must be here for aggregate query
	param.QuerySelectionSets = selectionSet

	driver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}

	result, err := driver.AggregateDocOfProject(p.Context, param)
	if err != nil {
		return nil, err
	}

	return result, nil
}

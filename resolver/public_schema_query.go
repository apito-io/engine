package resolver

import (
	"errors"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/tailor-platform/graphql"
	"github.com/vektah/gqlparser/v2/ast"
)

func (s *GraphQLServer) MultiResourceResolverFn(p graphql.ResolveParams) (interface{}, error) {

	cache, ok := utility.LegacyApplicationCache(p.Context)
	if !ok {
		return nil, errors.New("application cache missing in context")
	}
	rootSelectionSet, ok := utility.LegacySelectionSet(p.Context)
	if !ok {
		return nil, errors.New("selection set missing in context")
	}
	dbCtx := publicProjectDBContext(cache, p.Context)

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
		if utility.ModelIDMatchesGraphQLField(field.Name, model) {
			modelType = field
			break
		}
	}

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	if err := s.enforceRoleAgnosticModelRead(modelType.Name, cache.Param.Role); err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	param.Model = modelType
	param.ResolveParams = &p
	param.ProjectSchemaModels = cache.Project.Schema.Models

	param.QuerySelectionSets = selectionSet

	driver, err := s.GraphQLExecutor.GetProjectDriver(dbCtx)
	if err != nil {
		return nil, err
	}

	return driver.QueryMultiDocumentOfProject(dbCtx, param)
}

func (s *GraphQLServer) SingleResourceResolverFn(p graphql.ResolveParams) (interface{}, error) {

	cache, ok := utility.LegacyApplicationCache(p.Context)
	if !ok {
		return nil, errors.New("application cache missing in context")
	}
	rootSelectionSet, ok := utility.LegacySelectionSet(p.Context)
	if !ok {
		return nil, errors.New("selection set missing in context")
	}
	dbCtx := publicProjectDBContext(cache, p.Context)

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
		if utility.ModelIDMatchesGraphQLField(field.Name, model) {
			modelType = field
			break
		}
	}

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	if err := s.enforceRoleAgnosticModelRead(modelType.Name, cache.Param.Role); err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	param.Model = modelType
	param.ResolveParams = &p

	param.QuerySelectionSets = selectionSet

	if uid, ok := p.Args["_id"].(string); ok && uid != "" {
		param.DocumentID = uid
	} else if modelType.SinglePage {
		param.DocumentID = modelType.SinglePageUUID
	}

	if param.DocumentID == "" {
		return nil, errors.New("ID is required")
	}

	driver, err := s.GraphQLExecutor.GetProjectDriver(dbCtx)
	if err != nil {
		return nil, err
	}

	doc, err := driver.GetSingleProjectDocument(dbCtx, param)
	if err != nil {
		return nil, err
	}

	createdByID := ""
	if doc != nil && doc.Meta != nil && doc.Meta.CreatedBy != nil {
		createdByID = doc.Meta.CreatedBy.ID
	}
	// user documents: own means the document id is the authenticated user
	if doc != nil && doc.Type == "user" && createdByID == "" {
		createdByID = doc.ID
	}
	if err := utility.AuthorizeOwnDocumentRead(cache.Param.Role, modelType.Name, cache.Param.UserID, createdByID); err != nil {
		return nil, err
	}

	return doc, nil
}

func (s *GraphQLServer) CountResolverFn(p graphql.ResolveParams) (interface{}, error) {

	cache, ok := utility.LegacyApplicationCache(p.Context)
	if !ok {
		return nil, errors.New("application cache missing in context")
	}
	dbCtx := publicProjectDBContext(cache, p.Context)

	/*cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}*/

	pp := p.Source.(graphql.ResolveParams)

	model := utility.SingularResourceName(pp.Info.FieldName)

	var modelType *models.ModelType
	for _, field := range cache.Project.Schema.Models {
		if utility.ModelIDMatchesGraphQLField(field.Name, model) {
			modelType = field
			break
		}
	}

	if modelType == nil {
		return "", errors.New("Connection >  Model Type Not Found")
	}

	if err := s.enforceRoleAgnosticModelRead(modelType.Name, cache.Param.Role); err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)
	param.Model = modelType
	param.ResolveParams = &pp
	param.ProjectSchemaModels = cache.Project.Schema.Models
	param.OnlyReturnCount = true

	driver, err := s.GraphQLExecutor.GetProjectDriver(dbCtx)
	if err != nil {
		return nil, err
	}

	result, err := driver.CountDocOfProject(dbCtx, param)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *GraphQLServer) AggregateResolverFn(p graphql.ResolveParams) (interface{}, error) {

	cache, ok := utility.LegacyApplicationCache(p.Context)
	if !ok {
		return nil, errors.New("application cache missing in context")
	}
	rootSelectionSet, ok := utility.LegacySelectionSet(p.Context)
	if !ok {
		return nil, errors.New("selection set missing in context")
	}
	dbCtx := publicProjectDBContext(cache, p.Context)

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
		if utility.ModelIDMatchesGraphQLField(field.Name, model) {
			modelType = field
			break
		}
	}

	if modelType == nil {
		return "", errors.New("Connection >  Model Type Not Found")
	}

	if err := s.enforceRoleAgnosticModelRead(modelType.Name, cache.Param.Role); err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)
	param.Model = modelType
	param.ResolveParams = &pp
	param.ProjectSchemaModels = cache.Project.Schema.Models
	param.IsAggregateQuery = true

	// this is must be here for aggregate query
	param.QuerySelectionSets = selectionSet

	driver, err := s.GraphQLExecutor.GetProjectDriver(dbCtx)
	if err != nil {
		return nil, err
	}

	result, err := driver.AggregateDocOfProject(dbCtx, param)
	if err != nil {
		return nil, err
	}

	return result, nil
}

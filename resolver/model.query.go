package resolver

import (
	"errors"
	"strings"

	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	ae "github.com/apito-io/engine/err"
	"github.com/elliotchance/pie/pie"
	"github.com/jinzhu/inflection"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
)

func (s *GraphQLServer) ListModelsInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	project := cache.Project

	if project.Schema == nil {
		//return nil, errors.New(ae.SchemaIsNil)
		return []*protobuff.ModelType{}, nil
	}

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = strings.TrimSpace(inflection.Singular(val))
	}

	if modelName != "" {
		var modelType *protobuff.ModelType
		for _, model := range project.Schema.Models {
			if model.Name == modelName {
				modelType = model
				break
			}
		}

		if modelType == nil {
			return nil, ae.ModelTypeNotFound
		}

		// search and add locals
		var locals pie.Strings
		for _, f := range modelType.Fields {
			if f.Validation != nil {
				if len(f.Validation.Locals) > 0 {
					locals = append(locals, f.Validation.Locals...)
				}

				if f.Validation.IsSystemRole {
					f.Validation.FixedListElements = s.SystemRoles
				}
			}
		}

		modelType.Locals = locals.Unique()

		if len(modelType.Connections) > 0 {
			modelType.HasConnections = true
		}

		return []*protobuff.ModelType{modelType}, nil // resp is a list
	}

	for _, m := range project.Schema.Models {
		if len(m.Connections) > 0 {
			m.HasConnections = true
		}
	}

	return project.Schema.Models, nil
}

func (s *GraphQLServer) ProjectModelInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	doc := cache.Project

	var modelName string
	if val, ok := p.Args["model_name"].(string); ok {
		modelName = strings.TrimSpace(inflection.Singular(val))
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	if doc.Schema == nil {
		return nil, ae.SchemaIsNil
	}

	var modelType *protobuff.ModelType
	for _, model := range doc.Schema.Models {
		if model.Name == modelName {
			modelType = model
			break
		}
	}

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	// search and add locals
	var locals pie.Strings
	for _, f := range modelType.Fields {
		if f.Validation != nil {
			if len(f.Validation.Locals) > 0 {
				locals = append(locals, f.Validation.Locals...)
			}

			if f.Validation.IsSystemRole {
				f.Validation.FixedListElements = s.SystemRoles
			}
		}
	}

	modelType.Locals = locals.Unique()

	if len(modelType.Connections) > 0 {
		modelType.HasConnections = true
	}

	return modelType, nil
}

func (s *GraphQLServer) ListDetailedModelsDataProxyResolverFn(p graphql.ResolveParams) (interface{}, error) {
	return p, nil
}

func (s *GraphQLServer) ListDetailedModelsDataInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	// forward the proxy
	p.Args = p.Source.(graphql.ResolveParams).Args

	var modelName string
	if val, ok := p.Args["model"].(string); ok && val != "" {
		modelName = inflection.Singular(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var modelType *protobuff.ModelType
	for _, field := range cache.Project.Schema.Models {
		if field.Name == modelName {
			modelType = field
		}
	}

	param := s.NewParam(cache.Param)
	param.Model = modelType
	param.ResolveParams = &p

	param.IsSystemRequest = true

	return s.GraphQLExecutor.GetProjectDriver(ctx).QueryMultiDocumentOfProject(p.Context, *param)
}

func (s *GraphQLServer) ListDetailedModelsDataCountResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	// forward the proxy
	p.Args = p.Source.(graphql.ResolveParams).Args

	var modelName string
	if val, ok := p.Args["model"].(string); ok && val != "" {
		modelName = inflection.Singular(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var modelType *protobuff.ModelType
	for _, field := range cache.Project.Schema.Models {
		if field.Name == modelName {
			modelType = field
		}
	}

	param := s.NewParam(cache.Param)
	param.Model = modelType
	param.ResolveParams = &p

	param.IsSystemRequest = true

	return s.GraphQLExecutor.GetProjectDriver(ctx).CountMultiDocumentOfProject(p.Context, *param, false)
}

func (s *GraphQLServer) ListSingleModelDataInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	param.ResolveParams = &p

	if val, ok := p.Args["_id"].(string); ok {
		param.DocumentId = val
	} else {
		return nil, errors.New("ID is not provided")
	}

	if val, ok := p.Args["revision"].(bool); ok {
		param.Revision = val
	}

	var modelName string
	if val, ok := p.Args["model"].(string); ok && val != "" {
		modelName = inflection.Singular(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	var modelType *protobuff.ModelType
	for _, field := range cache.Project.Schema.Models {
		if field.Name == modelName {
			modelType = field
			break
		}
	}

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}
	param.Model = modelType

	if val, ok := p.Args["single_page_data"].(bool); ok {
		param.SinglePageData = val
	}

	// fetch document of all status
	p.Args["status"] = "all"

	param.IsSystemRequest = true

	doc, err := s.GraphQLExecutor.GetProjectDriver(ctx).GetSingleProjectDocument(p.Context, *param)
	if err != nil {
		return nil, err
	}

	// return empty data if single post data
	if doc.Id == "" && param.SinglePageData {
		return &shared.DefaultDocumentStructure{
			Key:      param.Model.SinglePageUuid,
			Id:       param.Model.SinglePageUuid,
			Type:     param.Model.Name,
			Data:     nil,
			ExpireAt: "",
		}, nil
	}

	// add the meta
	docWithMeta, err := s.SystemDriver.AddSystemUserMetaInfo(p.Context, doc)
	if err != nil {
		return nil, err
	}
	doc.Meta = docWithMeta.Meta

	return doc, nil
}

func (s *GraphQLServer) ListDocumentRevisionInfoResolverFn(p graphql.ResolveParams) (interface{}, error) {
	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	param := s.NewParam(cache.Param)

	param.ResolveParams = &p
	param.DocumentId = p.Args["_id"].(string)

	var modelName string
	if val, ok := p.Args["model"].(string); ok && val != "" {
		modelName = inflection.Singular(val)
	} else {
		return nil, errors.New(ae.MODEL_NAME_REQUIRED)
	}

	/*	if param.Plan == "free" {
		return []*models.DocumentRevisionHistory{}, nil
	}*/

	var modelType *protobuff.ModelType
	for _, field := range cache.Project.Schema.Models {
		if field.Name == modelName {
			modelType = field
			break
		}
	}

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	param.Model = modelType
	doc, err := s.GraphQLExecutor.GetProjectDriver(ctx).GetSingleProjectDocumentRevisions(p.Context, *param)
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func (s *GraphQLServer) ListSingleModelHasManyResolverFn(p graphql.ResolveParams) (interface{}, error) {

	s.Lock()
	defer s.Unlock()

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
		ctx    = p.Context
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	projectId := cache.Project.Id

	formModel := p.Args["from_model"].(string)

	toModel := p.Args["to_model"].(string)
	var modelType *protobuff.ModelType
	for _, model := range cache.Project.Schema.Models {
		if model.Name == toModel {
			modelType = model
			break
		}
	}

	if modelType == nil {
		return nil, ae.ModelTypeNotFound
	}

	id := p.Args["_id"].(string)

	param := &shared.CommonSystemParams{
		DocumentId:    id,
		ProjectId:     projectId,
		ResolveParams: &p,
		Model:         modelType,
	}

	if val, ok := p.Args["known_as"].(string); ok && val != "" {
		param.KnownAs = val
	}

	result, err := s.GraphQLExecutor.GetProjectDriver(ctx).GetAllRelationDocumentsOfSingleDocument(p.Context, formModel, param)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"data": result,
	}, nil
}

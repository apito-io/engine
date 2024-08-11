package resolver

import (
	"errors"
	"fmt"
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	"github.com/apito-io/engine/utility"
	"github.com/google/uuid"
	"github.com/tailor-inc/graphql"
)

func (s *GraphQLServer) MutationResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v = p.Context.Value
		//router = v("router").(echo.Context)
		cache = v("cache").(*shared.ApplicationCache)
	)

	/*cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}*/

	ctx := p.Context

	param := s.NewParam(cache.Param)

	collectionName := fmt.Sprintf("p_%s", param.ProjectId)
	fieldName := utility.ExtractResourceName(p.Info.FieldName)

	var modelType *protobuff.ModelType
	for _, model := range cache.Project.Schema.Models {
		if model.Name == fieldName {
			modelType = model
		}
	}

	if modelType == nil {
		return nil, errors.New("model Type Could Not be Found")
	}

	param.Model = modelType
	param.ResolveParams = &p

	action := utility.ExtractActionName(p.Info.FieldName)
	switch action {
	case "create":

		if modelType.SinglePage == true && modelType.SinglePageUuid != "" {
			return nil, errors.New(fmt.Sprintf("document with an id :%s already exists. create operation is not permitted. use update instead", modelType.SinglePageUuid))
		}

		// check connection first before creating
		if len(modelType.Connections) > 0 {
			if connections, ok := p.Args["connect"].(map[string]interface{}); ok {
				v, err := s.GraphQLExecutor.ConnectDisconnectParamBuilder(ctx, cache.Project.Schema, "", collectionName, connections, modelType)
				if err != nil {
					return nil, err
				}
				param.ConDisParam = v
				for _, _p := range v {
					isOneToOne, err := s.GraphQLExecutor.GetProjectDriver(ctx).CheckOneToOneRelationExists(ctx, _p)
					if err != nil {
						return nil, err
					}
					if isOneToOne {
						return nil, errors.New("The document that you are connecting with already has a one-to-one relation")
					}
				}
			}
		}

		fields := p.Args

		var status string
		if val, ok := fields["status"]; ok {
			status = val.(string)
		} else {
			status = "draft" // default is draft
		}

		uid := uuid.New()
		_id := uid.String()
		doc := shared.DefaultDocumentStructure{
			Key:  _id,
			Id:   _id,
			Type: fieldName,
			Meta: &protobuff.MetaField{
				CreatedAt: utility.GetCurrentTime(),
				UpdatedAt: utility.GetCurrentTime(),
				CreatedBy: &protobuff.SystemUser{
					Id: param.UserId,
				},
				LastModifiedBy: &protobuff.SystemUser{
					Id: param.UserId,
				},
				Status: status,
			},
		}

		if param.TenantId != "" {
			doc.Meta.TenantId = param.TenantId
		}

		if userInputPayload, ok := p.Args["payload"].(map[string]interface{}); ok && len(userInputPayload) > 0 {

			input := param.ResolveParams.Args
			var inputPayload map[string]interface{}
			if val, ok := input["payload"].(map[string]interface{}); ok {
				inputPayload = val
			}

			// local support
			local := "en"
			if val, ok := input["local"].(string); ok {
				local = val
			}

			//#todo need image param validation
			newPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(ctx, param, false, local, modelType.Fields, inputPayload, make(map[string]interface{}))
			if err != nil {
				return nil, err
			}
			doc.Data = newPayload
		} else {
			return nil, errors.New("can not create document with empty payload")
		}

		// create a new doc
		newDoc, err := s.GraphQLExecutor.GetProjectDriver(ctx).AddDocumentToProject(ctx, param.ProjectId, fieldName, &doc)
		if err != nil {
			return nil, err
		}

		if len(modelType.Connections) > 0 {

			// there is no disconnect builder in create !
			if connections, ok := p.Args["connect"].(map[string]interface{}); ok {
				v, err := s.GraphQLExecutor.ConnectDisconnectParamBuilder(ctx, cache.Project.Schema, doc.Id, collectionName, connections, modelType)
				if err != nil {
					return nil, err
				}
				param.ConDisParam = v
				err = s.GraphQLExecutor.GetProjectDriver(ctx).ConnectBuilder(ctx, *param)
				if err != nil {
					return nil, err
				}
			}
		}

		// fetch the doc again with query
		param.DocumentId = newDoc.(*shared.DefaultDocumentStructure).Id
		createdDoc, err := s.GraphQLExecutor.GetProjectDriver(ctx).GetSingleProjectDocument(ctx, *param)
		if err != nil {
			return nil, err
		}

		return createdDoc, nil
	case "update":
		fields := p.Args
		if val, ok := fields["_id"].(string); ok && val != "" {
			param.DocumentId = val
		} else {
			return nil, errors.New("Id is required for an update")
		}

		raw, err := s.GraphQLExecutor.GetProjectDriver(ctx).GetSingleRawDocumentFromProject(ctx, *param)
		if err != nil {
			return nil, err
		}
		doc := raw.(*shared.DefaultDocumentStructure)

		if userInputPayload, ok := p.Args["payload"].(map[string]interface{}); ok && len(userInputPayload) > 0 {

			input := param.ResolveParams.Args
			var inputPayload map[string]interface{}
			if val, ok := input["payload"].(map[string]interface{}); ok {
				inputPayload = val
			}

			// local support
			local := "en"
			if val, ok := input["local"].(string); ok {
				local = val
			}

			//#todo need image param validation
			updatedPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(ctx, param, false, local, modelType.Fields, inputPayload, doc.Data)
			if err != nil {
				return nil, err
			}
			doc.Data = updatedPayload
		}

		// update the meta
		doc.Meta.UpdatedAt = utility.GetCurrentTime()
		doc.Meta.LastModifiedBy.Id = param.UserId

		err = s.GraphQLExecutor.GetProjectDriver(ctx).UpdateDocumentOfProject(ctx, *param, doc, false)
		if err != nil {
			return nil, err
		}

		if len(modelType.Connections) > 0 {
			if disconnects, ok := p.Args["disconnect"].(map[string]interface{}); ok {
				v, err := s.GraphQLExecutor.ConnectDisconnectParamBuilder(ctx, cache.Project.Schema, param.DocumentId, collectionName, disconnects, modelType)
				param.ConDisParam = v
				err = s.GraphQLExecutor.GetProjectDriver(ctx).DisconnectBuilder(ctx, *param)
				if err != nil {
					return nil, err
				}
			}

			if connections, ok := p.Args["connect"].(map[string]interface{}); ok {
				v, err := s.GraphQLExecutor.ConnectDisconnectParamBuilder(ctx, cache.Project.Schema, param.DocumentId, collectionName, connections, modelType)
				param.ConDisParam = v
				err = s.GraphQLExecutor.GetProjectDriver(ctx).ConnectBuilder(ctx, *param)
				if err != nil {
					return nil, err
				}
			}
		}

		// fetch the doc again with query
		doc, err = s.GraphQLExecutor.GetProjectDriver(ctx).GetSingleProjectDocument(ctx, *param)
		if err != nil {
			return nil, err
		}

		return doc, nil
	case "delete":
		fields := p.Args
		uids := fields["_ids"].([]interface{})
		var documentToDelete uint32
		for _, id := range uids {
			param.DocumentId = id.(string)

			err := s.GraphQLExecutor.GetProjectDriver(ctx).DeleteDocumentFromProject(ctx, *param)
			if err != nil {
				return nil, err
			}
			documentToDelete++
		}

		return map[string]interface{}{
			"response": "Documents has been Deleted along with its connections if there were any",
		}, nil
	default:
		return nil, errors.New("invalid Actions")
	}

}

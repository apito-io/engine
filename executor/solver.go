package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	gqlgen "github.com/99designs/gqlgen/graphql"
	_const "github.com/apito-io/engine/const"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/google/uuid"
	"github.com/graph-gophers/dataloader/v7"
	"github.com/liangyaopei/structmap"
	"github.com/tailor-platform/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"golang.org/x/crypto/bcrypt"
)

type DataloaderResultTyped[T any] struct {
	Result  []T  `json:"result"`
	Count   int  `json:"count"`
	Cached  bool `json:"cached"`
	HasMore bool `json:"hasMore"`
	Error   bool `json:"error"`
	Code    int  `json:"code"`
}

// handleError creates array of result with the same error repeated for as many items requested
func handleError[T any](itemsLength int, err error) []*dataloader.Result[T] {
	result := make([]*dataloader.Result[T], itemsLength)
	for i := 0; i < itemsLength; i++ {
		result[i] = &dataloader.Result[T]{Error: err}
	}
	return result
}

func (s *GraphQLExecutor) NewParam(_param *models.CommonSystemParams) *models.CommonSystemParams {
	param := new(models.CommonSystemParams)
	*param = *_param
	return param
}

func (s *GraphQLExecutor) DataLoaderHandler(ctx context.Context, keys []string) []*dataloader.Result[interface{}] {
	data := ctx.Value("dataloaderMeta").(map[string]interface{})

	var cache *models.ApplicationCache
	if val, ok := data["cache"].(*models.ApplicationCache); ok {
		cache = val
	}

	param := s.NewParam(cache.Param)
	//relationMeta := ctx.Value("relation_meta").(map[string]interface{})

	/*var resolveParams *graphql.ResolveParams
	if val, ok := relationMeta["resolveParam"].(*graphql.ResolveParams); ok {
		resolveParams = val
	}*/
	/*var selectionSet *ast.SelectionSet
	if val, ok := relationMeta["selectionSet"].(*ast.SelectionSet); ok {
		selectionSet = val
	}*/

	//paramSource := resolveParams.Source.(*shared.DefaultDocumentStructure)
	var metaData map[string]interface{}
	if val, ok := data["metaData"].(map[string]interface{}); ok && val != nil {
		metaData = val
	}

	to_model := utility.SingularResourceName(strings.ToLower(data["parentObj"].(string)))

	modelName := utility.SingularResourceName(strings.ToLower(data["respObj"].(string)))

	var modelType *models.ModelType
	for _, _model := range cache.Project.Schema.Models {
		if _model.Name == modelName {
			modelType = _model
		}
	}

	if modelType == nil {
		return handleError[interface{}](len(keys), ae.ModelTypeNotFound)
	}

	var relationType string
	if val, ok := metaData["relation"].(string); ok {
		relationType = val
	}

	var knownAs string
	if val, ok := metaData["known_as"].(string); ok {
		knownAs = val
	}

	connection := map[string]interface{}{
		"to_model":        to_model,  // issue
		"model":           modelName, // comment
		"relation_type":   relationType,
		"known_as":        knownAs,
		"connection_type": metaData["type"].(string),
	}

	param.Model = modelType
	param.KnownAs = knownAs
	//param.ResolveParams = resolveParams // overwrite the parent

	// Custom Resolver implementation
	rc := gqlgen.GetRootFieldContext(ctx)

	var dataloaderModel string
	switch relationType {
	case "has_many":
		dataloaderModel = utility.MultipleResourceName(modelName)
	case "has_one":
		dataloaderModel = utility.SingularResourceName(modelName)
	}

	var selectionSet ast.SelectionSet
	for _, _s := range rc.Field.SelectionSet {
		if knownAs != "" {
			dataloaderModel = knownAs
		}
		if val := _s.(*ast.Field); val.Name == dataloaderModel {
			selectionSet = val.SelectionSet
			break
		}
	}

	param.QuerySelectionSets = &selectionSet
	param.DocumentIDs = keys

	driver, err := s.GetProjectDriver(ctx)
	if err != nil {
		return handleError[interface{}](len(keys), err)
	}

	resultBytes, err := driver.RelationshipDataLoaderBytes(ctx, param, connection)
	if err != nil {
		return handleError[interface{}](len(keys), err)
	}

	var results DataloaderResultTyped[map[string]interface{}]
	err = json.Unmarshal(resultBytes, &results)
	if err != nil {
		return handleError[interface{}](len(keys), err)
	}

	var dataloaderResults []*dataloader.Result[interface{}]

	for _, key := range keys {
		result := results.Result[0]
		if vals, ok := result[key].([]interface{}); ok && len(vals) > 0 {
			_val := map[string]interface{}{
				"result":  vals,
				"count":   len(vals),
				"cached":  results.Cached,
				"hasMore": results.HasMore,
				"error":   results.Error,
				"code":    results.Code,
			}
			dataloaderResults = append(dataloaderResults, &dataloader.Result[interface{}]{Data: _val})
		} else {
			dataloaderResults = append(dataloaderResults, &dataloader.Result[interface{}]{Data: nil})
		}
	}

	return dataloaderResults
	//s.GetProjectDriver().
}

func (s *GraphQLExecutor) SolvePublicQuery(ctx context.Context, model string, args interface{}, selectionSet *ast.SelectionSet, cache *models.ApplicationCache) ([]byte, error) {

	_args, _ := structmap.StructToMap(args, "json", "")

	/*	cache, err := s.GetApplicationCache(router)
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
	param.ResolveParams = &graphql.ResolveParams{Args: _args}

	driver, err := s.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}

	if uid, id := _args["_id"].(string); id {
		param.DocumentID = uid
		param.QuerySelectionSets = selectionSet

		doc, err := driver.GetSingleProjectDocumentBytes(ctx, param)
		if err != nil {
			return nil, err
		}
		return doc, nil
	}

	if modelType.SinglePage {
		param.DocumentID = modelType.SinglePageUUID
		param.QuerySelectionSets = selectionSet

		doc, err := driver.GetSingleProjectDocumentBytes(ctx, param)
		if err != nil {
			return nil, err
		}
		return doc, nil
	}

	param.QuerySelectionSets = selectionSet
	doc, err := driver.QueryMultiDocumentOfProjectBytes(ctx, param)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *GraphQLExecutor) SolvePublicQueryCount(ctx context.Context, model string, args interface{}, cache *models.ApplicationCache) ([]byte, error) {

	_args, _ := structmap.StructToMap(args, "json", "")

	/*	cache, err := s.GetApplicationCache(router)
		if err != nil {
			return nil, err
		}*/

	model = utility.SingularResourceName(strings.TrimSuffix(model, "Count"))

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
	param.ResolveParams = &graphql.ResolveParams{Args: _args}

	driver, err := s.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}

	return driver.CountDocOfProjectBytes(ctx, param)
}

func (s *GraphQLExecutor) SolvePublicMutation(ctx context.Context, resolverName string, _id *string, _ids []*string, status *string, local *string, _userInputPayload interface{}, _connect interface{}, _disconnect interface{}, cache *models.ApplicationCache) ([]byte, error) {

	userInputPayload, _ := structmap.StructToMap(_userInputPayload, "json", "")
	connect, _ := structmap.StructToMap(_connect, "json", "")
	disconnect, _ := structmap.StructToMap(_disconnect, "json", "")

	/*	cache, err := s.GetApplicationCache(router)
		if err != nil {
			return nil, err
		}*/

	param := s.NewParam(cache.Param)

	if param.Role.ID == "" {
		return nil, errors.New("bad request. Reload the Page again")
	}

	/*tenantId := router.Get("tenant")
	switch param.Role.ID {
	case "tenant":
		if tenantId == nil {
			return nil, errors.New("unable to Identify the User")
		}
		param.TenantId = tenantId.(string)
		break
	}*/

	model := utility.ExtractResourceName(resolverName)

	var modelType *models.ModelType
	for _, _model := range cache.Project.Schema.Models {
		if _model.Name == model {
			modelType = _model
		}
	}

	if modelType == nil {
		return nil, errors.New("model Type Could Not be Found")
	}

	param.Model = modelType
	//param.ResolveParams = &p

	// p == "none" || p == "all" || p == "own" || p == "tenant" || p == "auth"
	var permission *models.APIPermission

	// filter based on roles
	if param.Role.ID != "admin" {
		if val, ok := param.Role.APIPermissions[modelType.Name]; ok && param.Role.APIPermissions != nil {
			permission = val
		}
	}

	/*var hooks []*protobuff.Webhook
	// call in the webhook
	for _, hookId := range modelType.HookIds {
		hook, err := s.SystemDriver.GetWebHook(ctx, param.ProjectId, hookId)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, hook)
	}*/

	// local support
	var _local string
	if local != nil {
		_local = *local
	} else {
		_local = "en"
	}

	var _status string
	if status != nil {
		_status = *status
	} else {
		_status = "draft" // default is draft
	}

	driver, err := s.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}

	action := utility.ExtractActionName(resolverName)
	switch action {
	case "create":
		if param.Role.ID != "admin" {
			switch permission.Create {
			case "none":
				return nil, errors.New("creation is not permitted")
			case "auth":
				if !param.Role.IsProjectUser {
					return nil, errors.New("authentication is required to Create a Document")
				}
			}
		}

		if modelType.SinglePage == true && modelType.SinglePageUUID != "" {
			return nil, errors.New(fmt.Sprintf("document with an id :%s already exists. so, create operation is not possible. use update instead", modelType.SinglePageUUID))
		}

		// check connection first before creating
		if connect != nil && len(modelType.Connections) > 0 {
			v, err := utility.ConnectDisconnectParamBuilder(cache.Project, "", connect, modelType)
			if err != nil {
				return nil, err
			}
			param.ConDisParam = v
			for _, _p := range v {
				isOneToOne, err := driver.CheckOneToOneRelationExists(ctx, _p)
				if err != nil {
					return nil, err
				}
				if isOneToOne {
					return nil, errors.New("the document that you are connecting with already has a one-to-one relation")
				}
			}
		}

		_uid := uuid.New().String()
		doc := types.DefaultDocumentStructure{
			Key:      _uid,
			ID:       _uid,
			Type:     model,
			TenantID: types.ID(param.TenantID),
			Meta: &types.MetaField{
				CreatedAt: utility.GetCurrentTime(),
				UpdatedAt: utility.GetCurrentTime(),
				CreatedBy: &types.SystemUser{
					ID:            param.UserID,
					IsProjectUser: param.Role.IsProjectUser,
				},
				LastModifiedBy: &types.SystemUser{
					ID:            param.UserID,
					IsProjectUser: param.Role.IsProjectUser,
				},
				Status: _status,
			},
		}

		/*if param.TenantId != "" {
			doc.Meta.TenantId = param.TenantId
		}*/

		if userInputPayload != nil && len(userInputPayload) > 0 {
			//#todo need image param validation
			newPayload, err := s.HandlePayloadFormatting(ctx, param, _local, modelType.Fields, userInputPayload, make(map[string]interface{}), false)
			if err != nil {
				return nil, err
			}
			doc.Data = newPayload
		} else {
			return nil, errors.New("can not create document with empty payload")
		}

		// create a new doc
		newDoc, err := driver.AddDocumentToProject(ctx, param, &doc)
		if err != nil {
			return nil, err
		}

		// update the count		if s.Param.Role.ID != "admin" {
		//			switch permission.Create {
		//			case "none":
		//				return nil, errors.New("Creation is not permitted")
		//			case "auth":
		//				if !s.Param.IsProjectUser {
		//					return nil, errors.New("Authentication is required to Create a Document")
		//				}
		//			}
		//		}
		//
		//		// check connection first before creating
		//		if len(modelType.Connections) > 0 {
		//			if connections, ok := p.Args["connect"].(map[string]interface{}); ok {
		//				v, err := s.ConnectDisconnectParamBuilder("", collectionName, connections, modelType)
		//				if err != nil {
		//					return nil, err
		//				}
		//				param.ConDisParam = v
		//				for _, p := range v {
		//					isOneToOne, err := s.GetProjectDriver().CheckOneToOneRelationExists(p)
		//					if err != nil {
		//						return nil, err
		//					}
		//					if isOneToOne {
		//						return nil, errors.New("The document that you are connecting with already has a one-to-one relation")
		//					}
		//				}
		//			}
		//		}
		//
		//		fields := p.Args
		//
		//		var status string
		//		if val, ok := fields["status"]; ok {
		//			status = val.(string)
		//		} else {
		//			status = "draft" // default is draft
		//		}
		//
		//		var doc shared.DefaultDocumentStructure
		//		uid := uuid.New()
		//		doc.Key = uid.String()
		//		doc.ID = doc.Key
		//		doc.Type = fieldName
		//		doc.TenantId = s.Param.TenantId
		//		doc.Data = fields["payload"].(map[string]interface{})
		//		doc.Meta = &protobuff.MetaField{
		//			CreatedAt: utility.GetCurrentTime(),
		//			UpdatedAt: utility.GetCurrentTime(),
		//			CreatedBy: &protobuff.UserMeta{
		//				Id:          s.Param.UserId,
		//				ProjectUser: s.Param.IsProjectUser,
		//			},
		//			LastModifiedBy: &protobuff.UserMeta{
		//				Id:          s.Param.UserId,
		//				ProjectUser: s.Param.IsProjectUser,
		//			},
		//			Status: status,
		//		}
		//
		//		payload, _, err := s.HandlePayloadFormatting(doc.Data)
		//		if err != nil {
		//			return nil, err
		//		}
		//		doc.Data = payload
		//
		//		updatedDoc := doc
		//
		//		switch param.Role.ID {
		//		case "tenant":
		//			updatedDoc.Meta.TenantId = param.TenantId
		//			break
		//		}
		//
		//		// create a new doc
		//		_, err = s.GetProjectDriver().AddDocumentToProject(param.ProjectId, fieldName, &updatedDoc)
		//		if err != nil {
		//			return nil, err
		//		}
		//
		//		// update the count
		//		usage, err := s.SystemDriver.GetProjectUsages(param.ProjectId, nil)
		//		if err != nil {
		//			return nil, err
		//		}
		//
		//		inMb := (float64(unsafe.Sizeof(doc)) / 1024.0) / 1024.0 // in mb
		//		usage.Usages.ApiBandwidth = usage.Usages.ApiBandwidth + inMb
		//		usage.Usages.ApiCalls++
		//		usage.Usages.NumberOfRecords++
		//		err = s.SystemDriver.UpdateProjectUsagesDoc(usage, false)
		//		if err != nil {
		//			return nil, err
		//		}
		//
		//		if len(modelType.Connections) > 0 {
		//
		//			// there is no disconnect builder in create !
		//			if connections, ok := p.Args["connect"].(map[string]interface{}); ok {
		//				v, err := s.ConnectDisconnectParamBuilder(updatedDoc.ID, collectionName, connections, modelType)
		//				if err != nil {
		//					return nil, err
		//				}
		//				param.ConDisParam = v
		//				err = s.GetProjectDriver().ConnectBuilder(*param)
		//				if err != nil {
		//					return nil, err
		//				}
		//			}
		//		}
		//
		//		// if hook has actions
		//		for _, h := range hooks {
		//			if contains(h.Events, "create") {
		//				if h.Url != "" {
		//					go s.runWebHook("create", h, updatedDoc)
		//				}
		//				if len(h.LogicExecutions) > 0 {
		//					for _, t := range h.LogicExecutions {
		//						for _, f := range s.ProjectRawSchemas.Functions {
		//							if f.Name == t {
		//								go s.triggerFunction(f, "create", h, updatedDoc)
		//								break
		//							}
		//						}
		//					}
		//				}
		//			}
		//		}
		//
		//		return &updatedDoc, nil

		/*usage, err := s.SystemDriver.GetProjectUsages(ctx, param.ProjectId, nil)
		if err != nil {
			return nil, err
		}

		inMb := (float64(unsafe.Sizeof(doc)) / 1024.0) / 1024.0 // in mb
		usage.Usages.ApiBandwidth = usage.Usages.ApiBandwidth + inMb
		usage.Usages.ApiCalls++
		usage.Usages.NumberOfRecords++
		err = s.SystemDriver.UpdateProjectUsagesDoc(ctx, usage, true)
		if err != nil {
			return nil, err
		}*/

		if len(modelType.Connections) > 0 {

			// there is no disconnect builder in create !
			if connect != nil {
				v, err := utility.ConnectDisconnectParamBuilder(cache.Project, doc.ID, connect, modelType)
				if err != nil {
					return nil, err
				}
				param.ConDisParam = v
				err = driver.ConnectBuilder(ctx, param)
				if err != nil {
					return nil, err
				}
			}
		}

		// if hook has actions
		/*for _, h := range hooks {
			if utility.ArrayContains(h.Events, "create") {
				if h.Url != "" {
					go s.runWebHook("create", h, doc)
				}
				if len(h.LogicExecutions) > 0 {
					for _, t := range h.LogicExecutions {
						for _, f := range cache.Project.Schema.Functions {
							if f.Name == t {
								go s.triggerFunction(f, "create", h, doc)
								break
							}
						}
					}
				}
			}
		}*/

		// fetch the doc again with query
		param.DocumentID = newDoc.(*types.DefaultDocumentStructure).ID
		createdDoc, err := driver.GetSingleProjectDocumentBytes(ctx, param)
		if err != nil {
			return nil, err
		}
		return createdDoc, nil
	case "update":

		if _id != nil {
			param.DocumentID = *_id
		} else {
			return nil, errors.New("id is required for an update")
		}

		raw, err := driver.GetSingleRawDocumentFromProject(ctx, param)
		if err != nil {
			return nil, err
		}
		doc := raw.(*types.DefaultDocumentStructure)

		if param.Role.ID != "admin" {
			switch permission.Update {
			case "none":
				return nil, errors.New("update is not permitted")
			case "auth":
				if !param.Role.IsProjectUser {
					return nil, errors.New("authentication is required to Update a Document")
				}
				break
			case "own":
				if doc.Type == "user" && param.Role.IsProjectUser && param.UserID != doc.ID {
					return nil, errors.New("you are not authorized to edit this document")
				} else if doc.Meta.CreatedBy.IsProjectUser && doc.Meta.CreatedBy.ID != param.UserID {
					return nil, errors.New("you are not authorized to edit this document")
				}
			}
		}

		if userInputPayload != nil && len(userInputPayload) > 0 {

			input := param.ResolveParams.Args
			var inputPayload map[string]interface{}
			if val, ok := input["payload"].(map[string]interface{}); ok {
				inputPayload = val
			}

			//#todo need image param validation
			updatedPayload, err := s.HandlePayloadFormatting(ctx, param, _local, modelType.Fields, inputPayload, doc.Data, false)
			if err != nil {
				return nil, err
			}
			doc.Data = updatedPayload
		}

		// update the meta
		doc.Meta.UpdatedAt = utility.GetCurrentTime()
		doc.Meta.LastModifiedBy.ID = param.UserID
		doc.Meta.LastModifiedBy.IsProjectUser = param.Role.IsProjectUser

		err = driver.UpdateDocumentOfProject(ctx, param, doc, false)
		if err != nil {
			return nil, err
		}

		if len(modelType.Connections) > 0 {
			if disconnect != nil {
				v, err := utility.ConnectDisconnectParamBuilder(cache.Project, param.DocumentID, disconnect, modelType)
				if err != nil {
					return nil, err
				}
				param.ConDisParam = v
				err = driver.DisconnectBuilder(ctx, param)
				if err != nil {
					return nil, err
				}
			}

			if connect != nil {
				v, err := utility.ConnectDisconnectParamBuilder(cache.Project, param.DocumentID, connect, modelType)
				if err != nil {
					return nil, err
				}
				param.ConDisParam = v
				err = driver.ConnectBuilder(ctx, param)
				if err != nil {
					return nil, err
				}
			}
		}

		// if hook has actions
		/*for _, h := range hooks {
			if utility.ArrayContains(h.Events, "update") {
				if h.Url != "" {
					go s.runWebHook("update", h, doc)
				}
				if len(h.LogicExecutions) > 0 {
					for _, t := range h.LogicExecutions {
						for _, f := range cache.Project.Schema.Functions {
							if f.Name == t {
								go s.triggerFunction(f, "update", h, doc)
								break
							}
						}
					}
				}
			}
		}*/

		// fetch the doc again with query
		_doc, err := driver.GetSingleProjectDocumentBytes(ctx, param)
		if err != nil {
			return nil, err
		}

		return _doc, nil
	case "delete":

		var documentToDelete uint32
		for _, id := range _ids {
			param.DocumentID = *id
			doc, err := driver.GetSingleProjectDocument(ctx, param)
			if err != nil {
				return nil, err
			}

			if param.Role.ID != "admin" {
				switch permission.Delete {
				case "none":
					return nil, errors.New("update is not permitted")
				case "auth":
					if !param.Role.IsProjectUser {
						return nil, errors.New("Authentication is required to Update a Document")
					}
					break
				case "own":
					if doc.Type == "user" && param.Role.IsProjectUser && param.UserID != doc.ID {
						return nil, errors.New("you are not authorized to delete this document")
					} else if doc.Meta.CreatedBy.IsProjectUser && doc.Meta.CreatedBy.ID != param.UserID {
						return nil, errors.New("you are not authorized to delete this document")
					}
				}
			}

			err = driver.DeleteDocumentFromProject(ctx, param)
			if err != nil {
				return nil, err
			}
			documentToDelete++
		}

		// update the count
		/*usage, err := s.SystemDriver.GetProjectUsages(ctx, param.ProjectId, nil)
		if err != nil {
			return nil, err
		}

		usage.Usages.ApiCalls = usage.Usages.ApiCalls + documentToDelete
		usage.Usages.NumberOfRecords = usage.Usages.NumberOfRecords - float64(documentToDelete)
		err = s.SystemDriver.UpdateProjectUsagesDoc(ctx, usage, true)
		if err != nil {
			return nil, err
		}

		// if hook has actions
		for _, h := range hooks {
			if utility.ArrayContains(h.Events, "delete") {
				if h.Url != "" {
					go s.runWebHook("delete", h, _ids)
				}
				if len(h.LogicExecutions) > 0 {
					for _, t := range h.LogicExecutions {
						for _, f := range cache.Project.Schema.Functions {
							if f.Name == t {
								go s.triggerFunction(f, "delete", h, _ids)
								break
							}
						}
					}
				}
			}
		}*/
		return []byte("resources successfully deleted"), nil
	default:
		return nil, errors.New("invalid Actions")
	}
}

func (s *GraphQLExecutor) HandlePayloadFormatting(ctx context.Context, param *models.CommonSystemParams, local string,
	fields []*models.FieldInfo,
	inputPayload map[string]interface{},
	dbPayload map[string]interface{},
	deltaUpdate bool,
) (map[string]interface{}, error) {

	for _, f := range fields { // loop through the fields to format the payload

		// local support
		identifier := f.Identifier
		if f.Validation != nil && utility.ArrayContains(f.Validation.Locals, local) && local != "en" {
			identifier = fmt.Sprintf(`%s_%s`, f.Identifier, local)
		}

		switch f.FieldType { // special formatting
		case _const.BooleanField:

			if userInput, ok := inputPayload[f.Identifier].(bool); ok {
				dbPayload[identifier] = userInput // even if the string is empty insert it
			}

		case _const.TextField:

			if userInput, ok := inputPayload["secret"].(string); ok && param.Model.Name == "user" && f.Identifier == "secret" { // check for password payload.
				hash, err := bcrypt.GenerateFromPassword([]byte(userInput), bcrypt.DefaultCost)
				if err != nil {
					return nil, errors.New("Internal Error while saving secret")
				}
				dbPayload[identifier] = string(hash)
			} else {
				if userInput, ok := inputPayload[f.Identifier].(string); ok {
					dbPayload[identifier] = userInput // even if the string is empty insert it
				}
			}

		case _const.DateField:

			if val, ok := inputPayload[f.Identifier]; ok {
				if val == nil {
					dbPayload[identifier] = nil
				} else {
					switch val := val.(type) {
					case string:
						userInput := val
						_parsedTime, err := time.Parse(time.RFC3339, userInput)
						if err != nil {
							var parseError *time.ParseError
							if errors.As(err, &parseError) {
								_parsedTime, err = time.Parse("2006-01-02 15:04:05 -0700 MST", userInput)
								if err != nil {
									return nil, errors.New("invalid date input")
								}
								dbPayload[identifier] = _parsedTime.Format(time.RFC3339)
								continue
							}
						}
						//dbPayload[identifier] = _parsedTime.Add(6 * time.Hour).String() // even if the string is empty insert it || OLD FORMAT
						dbPayload[identifier] = _parsedTime.Format(time.RFC3339) // Use RFC3339 format without offset
					case interface{}:
						if val == nil {
							dbPayload[identifier] = nil
						}
					default:
						return nil, fmt.Errorf("invalid date input value for %s", f.Identifier)
					}
				}
			}

		case _const.MediaField:
			if userInput, ok := inputPayload[f.Identifier]; userInput != nil && ok {
				var err error
				switch reflect.ValueOf(userInput).Kind() {
				case reflect.Slice: // it could be multiple
					var result []interface{}
					for _, media := range userInput.([]interface{}) {
						var m interface{}
						m, err = s.HandleMediaURL(ctx, media.(map[string]interface{}))
						result = append(result, m)
					}
					if len(result) > 0 {
						dbPayload[identifier] = result
					}
				case reflect.Map: // it could be single
					if media, ok := userInput.(map[string]interface{}); ok {
						dbPayload[identifier], err = s.HandleMediaURL(ctx, media)
					}
				case reflect.String:
					if userInput.(string) == "" { // it means user has removed the image
						delete(dbPayload, identifier)
					}
				default:
					panic("unhandled default case")
				}
				if err != nil {
					return nil, err
				}
			}

		case _const.MultilineField:
			if userInput, ok := inputPayload[f.Identifier].(map[string]interface{}); ok && len(userInput) > 0 {
				// Process multiline field with markdown as source of truth
				processed := utility.ProcessMultilineField(userInput)
				if len(processed) > 0 {
					dbPayload[identifier] = processed
				}
			}
		case _const.ListField:

			if f.Validation != nil && f.Validation.FixedListElements == nil { // dynamic string
				if userInput, ok := inputPayload[f.Identifier].([]interface{}); ok {
					if len(userInput) == 0 { // if the user input is empty, set the db payload to nil
						dbPayload[identifier] = nil
					} else {
						dbPayload[identifier] = userInput
					}
				}
			} else if f.Validation != nil && f.Validation.FixedListElements != nil { // fixed list
				if userInput, ok := inputPayload[f.Identifier]; ok && userInput != nil {
					switch userInput := userInput.(type) {
					case string: // sinngle choise dropdown or radio button
						// validate the user input based on the fixed list elements
						if utility.ArrayContainsInterface(f.Validation.FixedListElements, userInput) {
							dbPayload[identifier] = userInput // assuming the choise is string type
						} else {
							return nil, fmt.Errorf("invalid dropdown value %v for %s", userInput, f.Identifier)
						}
					case []interface{}: // multi choise checkbox
						var result []string // assuming the choise is string type
						for _, item := range userInput {
							if utility.ArrayContainsInterface(f.Validation.FixedListElements, item) {
								result = append(result, item.(string))
							}
						}
						dbPayload[identifier] = result
					default:
						return nil, fmt.Errorf("invalid dropdown value %v for %s", userInput, f.Identifier)
					}
				}
			} else {
				if val, ok := inputPayload[f.Identifier].([]interface{}); ok {
					dbPayload[identifier] = val
				}
			}

		case _const.RepeatedField:

			if userInput, ok := inputPayload[f.Identifier].([]interface{}); ok && len(userInput) > 0 {

				// on delta update we update using the _id
				// delte operation is ambigous here so this mode doesnt support delete operation only update
				if deltaUpdate {

					userInputLength := len(userInput)
					var err error
					for i := 0; i < userInputLength; i++ {
						var repeatedUserInput map[string]interface{}

						repeatedUserInput = userInput[i].(map[string]interface{})

						var _currentID string
						if id, ok := repeatedUserInput["_id"].(string); ok && id != "" { // if _id exists on the user input
							_currentID = id
						} else {
							id = uuid.New().String()
							repeatedUserInput["_id"] = id
						}

						if _currentID != "" {
							if oldVals, ok := dbPayload[f.Identifier].([]interface{}); ok && len(oldVals) > 0 {
								for j, _ov := range oldVals {
									ov := _ov.(map[string]interface{})
									if _oldID, ok := ov["_id"].(string); ok && _oldID == _currentID {
										repeatedUserInput, err = s.HandlePayloadFormatting(ctx, param, local, f.SubFieldInfo, repeatedUserInput, ov, deltaUpdate)
										if err != nil {
											return nil, err
										}
										oldVals[j] = repeatedUserInput
										break
									}
								}
							} else {
								return nil, errors.New("old value not found ! where is the _id is from ?")
							}
						} else {
							repeatedUserInput, err = s.HandlePayloadFormatting(ctx, param, local, f.SubFieldInfo, repeatedUserInput, make(map[string]interface{}), deltaUpdate)
							if err != nil {
								return nil, err
							}
							if val, ok := dbPayload[f.Identifier].([]interface{}); ok && len(val) > 0 {
								dbPayload[identifier] = append(val, repeatedUserInput)
							} else { // if the field is empty, set the db payload to the new value
								dbPayload[f.Identifier] = []interface{}{repeatedUserInput}
							}
						}
					}
				} else {
					// this is normal update mode
					// it usages total replace so update, delete all supported but
					// be sure that you have to send the entire array of objects
					userInputLength := len(userInput)
					var formattedInput []interface{}
					var err error
					for i := 0; i < userInputLength; i++ {
						var repeatedUserInput map[string]interface{}

						repeatedUserInput = userInput[i].(map[string]interface{})

						var _currentID string
						if id, ok := repeatedUserInput["_id"].(string); ok { // if found
							_currentID = id
						} else {
							id = uuid.New().String()
							repeatedUserInput["_id"] = id
						}
						var oldUserInput map[string]interface{}
						if oldVals, ok := dbPayload[f.Identifier].([]interface{}); ok {
							for _, _ov := range oldVals {
								ov := _ov.(map[string]interface{})
								if _oldID, ok := ov["_id"].(string); ok && _oldID == _currentID {
									oldUserInput = ov
									break
								}
							}
							if oldUserInput == nil { // assuming not found or old array with no _id field
								oldUserInput = make(map[string]interface{})
							}
						} else {
							oldUserInput = make(map[string]interface{}) // assign an empty one
						}

						repeatedUserInput, err = s.HandlePayloadFormatting(ctx, param, local, f.SubFieldInfo, repeatedUserInput, oldUserInput, deltaUpdate)
						if err != nil {
							return nil, err
						}
						formattedInput = append(formattedInput, repeatedUserInput)
					}
					dbPayload[identifier] = formattedInput
				}
			}
		case _const.ObjectField:
			if userInput, ok := inputPayload[f.Identifier].(map[string]interface{}); ok && len(userInput) > 0 {
				var oldUserInput map[string]interface{}
				if val, ok := dbPayload[f.Identifier].(map[string]interface{}); ok {
					oldUserInput = val
				} else {
					oldUserInput = make(map[string]interface{}) // assign an empty one
				}
				repeatedUserInput, err := s.HandlePayloadFormatting(ctx, param, local, f.SubFieldInfo, userInput, oldUserInput, deltaUpdate)
				if err != nil {
					return nil, err
				}
				dbPayload[identifier] = repeatedUserInput
			}
		case _const.NumberField:
			switch f.InputType {
			case "int":
				switch inputPayload[f.Identifier].(type) {
				case int:
					if userInput, ok := inputPayload[f.Identifier].(int); ok {
						_int := int(userInput)
						if _int > 2147483647 {
							return nil, fmt.Errorf("integer value Overflow for `%s`. Field type is signed Int.\nTo store large number, try Double field instead", strings.ToTitle(identifier))
						}
						dbPayload[identifier] = _int
					}
				case float64:
					if val, ok := inputPayload[f.Identifier].(float64); ok {
						dbPayload[identifier] = int(val)
					}
				}
			default: // for double
				if val, ok := inputPayload[f.Identifier]; ok {
					dbPayload[identifier] = val
				}
			}
		case _const.GeoField:
			if userInput, ok := inputPayload[f.Identifier].(map[string]interface{}); ok && len(userInput) > 0 { // expecting a map
				lat := userInput["lat"].(float64)
				lon := userInput["lon"].(float64)
				dbPayload[identifier] = map[string]interface{}{
					"lat":         lat,
					"lon":         lon,
					"type":        "Point",
					"coordinates": []float64{lat, lon},
				}
			}
		}
	}
	return dbPayload, nil
}

func (s *GraphQLExecutor) TrackUploadHistory(ctx context.Context, param *models.CommonSystemParams, sent *models.FileDetails) error {

	driver, err := s.GetProjectDriver(ctx)
	if err != nil {
		return err
	}

	// get the id
	if sent.UploadParam != nil && sent.UploadParam.DocID != "" {
		doc, err := driver.GetSingleProjectDocument(ctx, &models.CommonSystemParams{
			ProjectID:  param.ProjectID,
			DocumentID: sent.UploadParam.DocID,
		})
		if err != nil {
			return err
		}

		// update the field value
		doc.Data[sent.UploadParam.FieldName] = map[string]interface{}{
			"file_name": sent.FileName,
			"id":        sent.ID,
			"url":       sent.URL,
		}
		err = driver.UpdateDocumentOfProject(ctx, param, doc, false)
		if err != nil {
			return err
		}
	}

	/*projectUsages, err := u.SystemDriver.GetProjectUsages(ctx, param.ProjectId, nil)
	if err != nil {
		return err
	}

	inMb := (float64(sent.Size) / 1024.0) / 1024.0 // in mb
	projectUsages.Usages.MediaStorage = projectUsages.Usages.MediaStorage + inMb
	projectUsages.Usages.NumberOfMedia++
	projectUsages.Usages.ApiCalls++
	err = u.SystemDriver.UpdateProjectUsagesDoc(ctx, projectUsages, true)
	if err != nil {
		return err
	}*/
	return nil
}

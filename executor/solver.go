package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/graph-gophers/dataloader/v7"

	"reflect"
	"strings"
	"time"

	gqlgen "github.com/99designs/gqlgen/graphql"
	"github.com/apito-io/buffers/protobuff"
	"github.com/apito-io/buffers/shared"
	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/engine/faker"
	"github.com/apito-io/engine/utility"
	"github.com/google/uuid"
	"github.com/jinzhu/inflection"
	"github.com/liangyaopei/structmap"
	"github.com/microcosm-cc/bluemonday"
	"github.com/tailor-inc/graphql"
	"github.com/vektah/gqlparser/v2/ast"
	"golang.org/x/crypto/bcrypt"
	faker2 "syreclabs.com/go/faker"
)

// handleError creates array of result with the same error repeated for as many items requested
func handleError[T any](itemsLength int, err error) []*dataloader.Result[T] {
	result := make([]*dataloader.Result[T], itemsLength)
	for i := 0; i < itemsLength; i++ {
		result[i] = &dataloader.Result[T]{Error: err}
	}
	return result
}

func (s *GraphQLExecutor) NewParam(_param *shared.CommonSystemParams) *shared.CommonSystemParams {
	param := new(shared.CommonSystemParams)
	*param = *_param
	return param
}

func (s *GraphQLExecutor) DataLoaderHandlr(ctx context.Context, keys []string) []*dataloader.Result[interface{}] {
	data := ctx.Value("dataloaderMeta").(map[string]interface{})

	var cache *shared.ApplicationCache
	if val, ok := data["cache"].(*shared.ApplicationCache); ok {
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

	to_model := inflection.Singular(strings.ToLower(data["parentObj"].(string)))

	modelName := inflection.Singular(strings.ToLower(data["respObj"].(string)))

	var modelType *protobuff.ModelType
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
		dataloaderModel = inflection.Plural(modelName)
	case "has_one":
		dataloaderModel = inflection.Singular(modelName)
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

	resultBytes, err := s.GetProjectDriver(ctx).RelationshipDataLoaderBytes(ctx, param, connection)
	if err != nil {
		return handleError[interface{}](len(keys), err)
	}

	var results shared.DataloaderResult[map[string]interface{}]
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

func (s *GraphQLExecutor) SolvePublicQuery(ctx context.Context, model string, args interface{}, selectionSet *ast.SelectionSet, cache *shared.ApplicationCache) ([]byte, error) {

	_args, _ := structmap.StructToMap(args, "json", "")

	/*	cache, err := s.GetApplicationCache(router)
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
	param.ResolveParams = &graphql.ResolveParams{Args: _args}

	if uid, id := _args["_id"].(string); id {
		param.DocumentId = uid
		param.QuerySelectionSets = selectionSet

		doc, err := s.GetProjectDriver(ctx).GetSingleProjectDocumentBytes(ctx, *param)
		if err != nil {
			return nil, err
		}
		return doc, nil
	}

	if modelType.SinglePage {
		param.DocumentId = modelType.SinglePageUuid
		param.QuerySelectionSets = selectionSet

		doc, err := s.GetProjectDriver(ctx).GetSingleProjectDocumentBytes(ctx, *param)
		if err != nil {
			return nil, err
		}
		return doc, nil
	}

	param.QuerySelectionSets = selectionSet
	return s.GetProjectDriver(ctx).QueryMultiDocumentOfProjectBytes(ctx, *param)
}

func (s *GraphQLExecutor) SolvePublicQueryCount(ctx context.Context, model string, args interface{}, cache *shared.ApplicationCache) ([]byte, error) {

	_args, _ := structmap.StructToMap(args, "json", "")

	/*	cache, err := s.GetApplicationCache(router)
		if err != nil {
			return nil, err
		}*/

	model = inflection.Singular(strings.TrimSuffix(model, "Count"))

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
	param.ResolveParams = &graphql.ResolveParams{Args: _args}

	return s.GetProjectDriver(ctx).CountDocOfProjectBytes(ctx, param)
}

func (s *GraphQLExecutor) SolvePublicMutation(ctx context.Context, resolverName string, _id *string, _ids []*string, status *string, local *string, _userInputPayload interface{}, _connect interface{}, _disconnect interface{}, cache *shared.ApplicationCache) ([]byte, error) {

	userInputPayload, _ := structmap.StructToMap(_userInputPayload, "json", "")
	connect, _ := structmap.StructToMap(_connect, "json", "")
	disconnect, _ := structmap.StructToMap(_disconnect, "json", "")

	/*	cache, err := s.GetApplicationCache(router)
		if err != nil {
			return nil, err
		}*/

	param := s.NewParam(cache.Param)

	/*tenantId := router.Get("tenant")
	switch param.Role.Id {
	case "tenant":
		if tenantId == nil {
			return nil, errors.New("unable to Identify the User")
		}
		param.TenantId = tenantId.(string)
		break
	}*/

	collectionName := fmt.Sprintf("p_%s", param.ProjectId)

	model := utility.ExtractResourceName(resolverName)

	var modelType *protobuff.ModelType
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

	action := utility.ExtractActionName(resolverName)
	switch action {
	case "create":

		if modelType.SinglePage && modelType.SinglePageUuid != "" {
			return nil, fmt.Errorf("document with an id :%s already exists. so, create operation is not possible. use update instead", modelType.SinglePageUuid)
		}

		// check connection first before creating
		if connect != nil && len(modelType.Connections) > 0 {
			v, err := s.ConnectDisconnectParamBuilder(ctx, cache.Project.Schema, "", collectionName, connect, modelType)
			if err != nil {
				return nil, err
			}
			param.ConDisParam = v
			for _, _p := range v {
				isOneToOne, err := s.GetProjectDriver(ctx).CheckOneToOneRelationExists(ctx, _p)
				if err != nil {
					return nil, err
				}
				if isOneToOne {
					return nil, errors.New("the document that you are connecting with already has a one-to-one relation")
				}
			}
		}

		_uid := uuid.New().String()
		doc := shared.DefaultDocumentStructure{
			Key:  _uid,
			Id:   _uid,
			Type: model,
			Meta: &protobuff.MetaField{
				CreatedAt: utility.GetCurrentTime(),
				UpdatedAt: utility.GetCurrentTime(),
				CreatedBy: &protobuff.SystemUser{
					Id: param.UserId,
					//ProjectUser: param.Role.IsProjectUser,
				},
				LastModifiedBy: &protobuff.SystemUser{
					Id: param.UserId,
					//ProjectUser: param.Role.IsProjectUser,
				},
				Status: _status,
			},
		}

		if param.TenantId != "" {
			doc.Meta.TenantId = param.TenantId
		}

		if userInputPayload != nil && len(userInputPayload) > 0 {
			//#todo need image param validation
			newPayload, err := s.HandlePayloadFormatting(ctx, param, false, _local, modelType.Fields, userInputPayload, make(map[string]interface{}))
			if err != nil {
				return nil, err
			}
			doc.Data = newPayload
		} else {
			return nil, errors.New("can not create document with empty payload")
		}

		// create a new doc
		newDoc, err := s.GetProjectDriver(ctx).AddDocumentToProject(ctx, param.ProjectId, model, &doc)
		if err != nil {
			return nil, err
		}

		if len(modelType.Connections) > 0 {

			// there is no disconnect builder in create !
			if connect != nil {
				v, err := s.ConnectDisconnectParamBuilder(ctx, cache.Project.Schema, doc.Id, collectionName, connect, modelType)
				if err != nil {
					return nil, err
				}
				param.ConDisParam = v
				err = s.GetProjectDriver(ctx).ConnectBuilder(ctx, *param)
				if err != nil {
					return nil, err
				}
			}
		}

		// fetch the doc again with query
		param.DocumentId = newDoc.(*shared.DefaultDocumentStructure).Id
		createdDoc, err := s.GetProjectDriver(ctx).GetSingleProjectDocumentBytes(ctx, *param)
		if err != nil {
			return nil, err
		}
		return createdDoc, nil
	case "update":

		if _id != nil {
			param.DocumentId = *_id
		} else {
			return nil, errors.New("id is required for an update")
		}

		raw, err := s.GetProjectDriver(ctx).GetSingleRawDocumentFromProject(ctx, *param)
		if err != nil {
			return nil, err
		}
		doc := raw.(*shared.DefaultDocumentStructure)

		if userInputPayload != nil && len(userInputPayload) > 0 {

			input := param.ResolveParams.Args
			var inputPayload map[string]interface{}
			if val, ok := input["payload"].(map[string]interface{}); ok {
				inputPayload = val
			}

			//#todo need image param validation
			updatedPayload, err := s.HandlePayloadFormatting(ctx, param, false, _local, modelType.Fields, inputPayload, doc.Data)
			if err != nil {
				return nil, err
			}
			doc.Data = updatedPayload
		}

		// update the meta
		doc.Meta.UpdatedAt = utility.GetCurrentTime()
		doc.Meta.LastModifiedBy.Id = param.UserId
		//doc.Meta.LastModifiedBy.ProjectUser = param.Role.IsProjectUser

		err = s.GetProjectDriver(ctx).UpdateDocumentOfProject(ctx, *param, doc, false)
		if err != nil {
			return nil, err
		}

		if len(modelType.Connections) > 0 {
			if disconnect != nil {
				v, err := s.ConnectDisconnectParamBuilder(ctx, cache.Project.Schema, param.DocumentId, collectionName, disconnect, modelType)
				param.ConDisParam = v
				err = s.GetProjectDriver(ctx).DisconnectBuilder(ctx, *param)
				if err != nil {
					return nil, err
				}
			}

			if connect != nil {
				v, err := s.ConnectDisconnectParamBuilder(ctx, cache.Project.Schema, param.DocumentId, collectionName, connect, modelType)
				param.ConDisParam = v
				err = s.GetProjectDriver(ctx).ConnectBuilder(ctx, *param)
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
		_doc, err := s.GetProjectDriver(ctx).GetSingleProjectDocumentBytes(ctx, *param)
		if err != nil {
			return nil, err
		}

		return _doc, nil
	case "delete":

		var documentToDelete uint32
		for _, id := range _ids {
			param.DocumentId = *id
			/*doc, err := s.GetProjectDriver(ctx).GetSingleProjectDocument(ctx, *param)
			if err != nil {
				return nil, err
			}*/

			err := s.GetProjectDriver(ctx).DeleteDocumentFromProject(ctx, *param)
			if err != nil {
				return nil, err
			}
			documentToDelete++
		}

		return []byte("resources successfully deleted"), nil
	default:
		return nil, errors.New("invalid Actions")
	}
}

func (s *GraphQLExecutor) ConnectDisconnectParamBuilder(ctx context.Context, schema *protobuff.ProjectSchema, uid string, collectionName string, connectionIds map[string]interface{}, modelType *protobuff.ModelType) ([]*shared.ConnectDisconnectParam, error) {

	var connParms []*shared.ConnectDisconnectParam

	for k, v := range connectionIds {
		connParam := shared.ConnectDisconnectParam{}
		connParam.DocCollectionName = collectionName
		connParam.DocRelationName = fmt.Sprintf(`%s_relation`, collectionName)

		var ids []string
		var relationTo string
		if strings.HasSuffix(k, "_ids") {
			if val, ok := v.([]interface{}); ok {
				for _, id := range val {
					if id != nil {
						ids = append(ids, id.(string))
					}
				}
				relationTo = strings.TrimSuffix(k, "_ids")
			} else {
				return nil, errors.New("invalid Relation Input Type, Expected a List")
			}
		} else if strings.HasSuffix(k, "_id") {
			if val, ok := v.(string); ok {
				ids = append(ids, val)
				relationTo = strings.TrimSuffix(k, "_id")
			} else {
				return nil, errors.New("invalid Relation Input Value Type")
			}
		}

		var knownAs string
		var connValCheck *protobuff.ConnectionType
		// validate the relation to
		for _, c := range modelType.Connections {
			if c.Model == relationTo && c.KnownAs == "" {
				connValCheck = c
				knownAs = ""
				break
			} else if c.KnownAs == relationTo {
				connValCheck = c
				connParam.KnownAs = c.KnownAs
				relationTo = c.Model // restore the original name
				knownAs = c.KnownAs
				break
			}
		}

		if connValCheck != nil {
			switch connValCheck.Relation {
			case "has_many":
				if !strings.HasSuffix(k, "_ids") {
					return nil, errors.New("Has Many Relation doesnt support _id, try _ids instead")
				}
				break
			case "has_one":
				if !strings.HasSuffix(k, "_id") {
					return nil, errors.New("Has one Relation doesnt support _ids, try _id instead")
				}
				break
			}
		} else {
			return nil, errors.New(fmt.Sprintf("Invalid Relation %s", k))
		}

		// validate the relation to
		for _, connection := range modelType.Connections {
			if connection.Model == relationTo && connection.Type == "backward" && connection.KnownAs == knownAs {

				connParam.ForwardConnectionId = uid
				connParam.ConnectionIds = ids

				connParam.ConnectionType = connection.Type
				connParam.BackwardConnectionType = connection
				// find forward
				var forwardModelType *protobuff.ModelType
				for _, ct := range schema.Models {
					if ct.Name == relationTo {
						forwardModelType = ct
						break
					}
				}
				for _, _connection := range forwardModelType.Connections {
					if _connection.Model == modelType.Name && _connection.Type == "forward" {
						connParam.BackendConnectionModelType = forwardModelType
						connParam.ForwardConnectionType = _connection
					}
				}
				connParms = append(connParms, &connParam)
				fmt.Println(connParam)

			} else if connection.Model == relationTo && connection.Type == "forward" && connection.KnownAs == knownAs {

				connParam.ForwardConnectionId = uid
				connParam.ConnectionIds = ids

				connParam.ConnectionType = connection.Type
				connParam.ForwardConnectionType = connection
				// find forward
				var backwardModelType *protobuff.ModelType
				for _, ct := range schema.Models {
					if ct.Name == relationTo {
						backwardModelType = ct
						break
					}
				}
				for _, _connection := range backwardModelType.Connections {
					if _connection.Model == modelType.Name && _connection.Type == "backward" {
						connParam.BackendConnectionModelType = backwardModelType
						connParam.BackwardConnectionType = _connection
					}
				}
				connParms = append(connParms, &connParam)
			}
		}
	}

	return connParms, nil
}

func (s *GraphQLExecutor) HandlePayloadFormatting(
	ctx context.Context,
	param *shared.CommonSystemParams,
	isFaker bool, local string,
	fields []*protobuff.FieldInfo,
	inputPayload map[string]interface{},
	dbPayload map[string]interface{}) (map[string]interface{}, error) {

	for _, f := range fields { // loop through the fields to format the payload

		// local support
		identifier := f.Identifier
		if f.Validation != nil && utility.ArrayContains(f.Validation.Locals, local) && local != "en" {
			identifier = fmt.Sprintf(`%s_%s`, f.Identifier, local)
		}

		switch f.FieldType { // special formatting
		case "boolean":
			if isFaker {
				dbPayload[identifier] = false
			} else {
				if userInput, ok := inputPayload[f.Identifier].(bool); ok {
					dbPayload[identifier] = userInput // even if the string is empty insert it
				}
			}
			break
		case "text":
			if isFaker {
				if userInput, ok := inputPayload[f.Identifier].(string); ok {
					dbPayload[identifier] = faker.StringFormatter(userInput)
				}
			} else {
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
			}
			break
		case "date":
			if isFaker {
				if _, ok := inputPayload[f.Identifier].(string); ok {
					dbPayload[identifier] = faker2.Date()
				}
			} else {
				if val, ok := inputPayload[f.Identifier]; ok {
					if val == nil {
						dbPayload[identifier] = nil
					} else {
						switch val.(type) {
						case string:
							userInput := val.(string)
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
							dbPayload[identifier] = _parsedTime.Add(6 * time.Hour).String() // even if the string is empty insert it
						case interface{}:
							if val == nil {
								dbPayload[identifier] = nil
							}
						default:
							return nil, errors.New(fmt.Sprintf("invalid date input value for %s", f.Identifier))
						}
					}
				}
			}
			break
		case "media":
			if userInput := inputPayload[f.Identifier]; userInput != nil {
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
			break
		case "multiline":
			if userInput, ok := inputPayload[f.Identifier].(map[string]interface{}); ok && len(userInput) > 0 {
				if html, ok := userInput["html"].(string); ok {
					p := bluemonday.UGCPolicy()
					dbPayload[identifier] = map[string]interface{}{
						"html": p.Sanitize(faker.MultilineFormatter(isFaker, html)),
					}
				}
			}
			break
		case "list":
			if isFaker {
				dbPayload[identifier] = faker.ListFormatter(isFaker, f.Validation.IsMultiChoice, f.Validation.FixedListElements, inputPayload[f.Identifier])
			} else {
				if f.Validation != nil && f.Validation.FixedListElements == nil { // dynamic string
					if userInput, ok := inputPayload[f.Identifier].([]interface{}); ok && len(userInput) > 0 {
						dbPayload[identifier] = userInput
					}
				} else {
					dbPayload[identifier] = inputPayload[f.Identifier]
				}
			}
			break
		case "repeated":
			if userInput, ok := inputPayload[f.Identifier].([]interface{}); ok && len(userInput) > 0 {
				var userInputLength int
				if isFaker {
					if val, ok := userInput[0].(map[string]interface{}); ok {
						userInputLength = int(val["number_of_records"].(float64))
					} else {
						userInputLength = 5 // default is 5
					}
				} else {
					userInputLength = len(userInput)
				}

				var formattedInput []interface{}
				var err error
				for i := 0; i < userInputLength; i++ {
					var repeatedUserInput map[string]interface{}
					if isFaker {
						repeatedUserInput = userInput[0].(map[string]interface{})
					} else {
						repeatedUserInput = userInput[i].(map[string]interface{})
					}
					var _currentId string
					if id, ok := repeatedUserInput["_id"].(string); ok { // if found
						_currentId = id
					} else {
						id = uuid.New().String()
						repeatedUserInput["_id"] = id
					}
					var oldUserInput map[string]interface{}
					if oldVals, ok := dbPayload[f.Identifier].([]map[string]interface{}); ok {
						for _, ov := range oldVals {
							if _oldId, ok := ov["_id"].(string); ok && _oldId == _currentId {
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

					repeatedUserInput, err = s.HandlePayloadFormatting(ctx, param, isFaker, local, f.SubFieldInfo, repeatedUserInput, oldUserInput)
					if err != nil {
						return nil, err
					}
					formattedInput = append(formattedInput, repeatedUserInput)
				}
				dbPayload[identifier] = formattedInput
			}
			break
		case "object":
			if userInput, ok := inputPayload[f.Identifier].(map[string]interface{}); ok && len(userInput) > 0 {

				var oldUserInput map[string]interface{}
				if val, ok := dbPayload[f.Identifier].(map[string]interface{}); ok {
					oldUserInput = val
				} else {
					oldUserInput = make(map[string]interface{}) // assign an empty one
				}

				repeatedUserInput, err := s.HandlePayloadFormatting(ctx, param, isFaker, local, f.SubFieldInfo, userInput, oldUserInput)
				if err != nil {
					return nil, err
				}

				dbPayload[identifier] = repeatedUserInput
			}
			break
		case "number":
			if isFaker {
				dbPayload[identifier] = faker.NumberFormatter(inputPayload[f.Identifier])
			} else {
				switch f.InputType {
				case "int":
					if userInput, ok := inputPayload[f.Identifier].(float64); ok {
						_int := int(userInput)
						if _int > 2147483647 {
							return nil, errors.New(fmt.Sprintf("Int Value Overflow for `%s`. Field type is signed Int.\nTo store large number, try Double field instead", strings.ToTitle(identifier)))
						}
						dbPayload[identifier] = _int
					}
					break
				default: // for double
					dbPayload[identifier] = inputPayload[f.Identifier]
				}
			}
			break
		case "geo":
			if isFaker {
				dbPayload[identifier] = faker.GeoFormatter(inputPayload[f.Identifier])
			} else {
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
			break
		}
	}
	return dbPayload, nil
}

func (u *GraphQLExecutor) TrackUploadHistory(ctx context.Context, param *shared.CommonSystemParams, sent *protobuff.FileDetails) error {

	// get the id
	if sent.UploadParam != nil && sent.UploadParam.DocId != "" {
		doc, err := u.GetProjectDriver(ctx).GetSingleProjectDocument(ctx, shared.CommonSystemParams{
			ProjectId:  param.ProjectId,
			DocumentId: sent.UploadParam.DocId,
		})
		if err != nil {
			return err
		}

		// update the field value
		doc.Data[sent.UploadParam.FieldName] = map[string]interface{}{
			"file_name": sent.FileName,
			"id":        sent.Id,
			"url":       sent.Url,
		}
		err = u.GetProjectDriver(ctx).UpdateDocumentOfProject(ctx, *param, doc, false)
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

package resolver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/tailor-platform/graphql"
)

// roleBypassesMutationACL is true for project admin role id or any role with IsAdmin (e.g. owner merged from admin template in BuildSystemParam).
func roleBypassesMutationACL(r *models.Role) bool {
	return r != nil && (r.ID == "admin" || r.IsAdmin)
}

// mutationPermissionForRole returns non-nil API CRUD scopes for the model. LookupAPIPermission alone can miss
// (nil api_permissions on role); BuildCRUDPermissions supplies the same defaults as the rest of the stack.
func mutationPermissionForRole(role *models.Role, modelName string) (*models.APIPermission, error) {
	if role == nil {
		return nil, errors.New("role is required")
	}
	if roleBypassesMutationACL(role) {
		return &models.APIPermission{Read: "all", Create: "all", Update: "all", Delete: "all"}, nil
	}
	if val, ok := utility.LookupAPIPermission(role, modelName); ok && val != nil {
		return val, nil
	}
	return utility.BuildCRUDPermissions(modelName, role)
}

func (s *GraphQLServer) updateAndConnectDocument(ctx context.Context, cache *models.ApplicationCache, param *models.CommonSystemParams, hooks []*models.Webhook, userInputPayload map[string]interface{}, permission *models.APIPermission, connections, disconnects map[string]interface{}, deltaUpdate bool) (*types.DefaultDocumentStructure, error) {

	if len(userInputPayload) == 0 {
		return nil, errors.New("payload is required")
	}

	driver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}

	raw, err := driver.GetSingleRawDocumentFromProject(ctx, param)
	if err != nil {
		return nil, err
	}
	doc := raw.(*types.DefaultDocumentStructure)

	if !roleBypassesMutationACL(param.Role) {
		if permission == nil {
			return nil, errors.New("internal error: mutation permission is not resolved")
		}
		switch permission.Update {
		case "none":
			return nil, errors.New("Update is not permitted")
		case "auth":
			if !param.Role.IsProjectUser {
				return nil, errors.New("Authentication is required to Update a Document")
			}
		case "own":
			if doc.Type == "user" && param.Role.IsProjectUser && param.UserID != doc.ID {
				return nil, errors.New("You are not authorized to edit this document")
			} else if doc.Meta.CreatedBy.IsProjectUser && doc.Meta.CreatedBy.ID != param.UserID {
				return nil, errors.New("You are not authorized to edit this document")
			}
		}
	}

	input := param.ResolveParams.Args

	var inputPayload map[string]interface{}
	if len(userInputPayload) > 0 {
		inputPayload = userInputPayload
	} else {
		if val, ok := input["payload"].(map[string]interface{}); ok {
			inputPayload = val
		}
	}

	if len(inputPayload) == 0 {
		return nil, errors.New("user input payload is required")
	}

	// local support
	local := "en"
	if val, ok := input["local"].(string); ok {
		local = val
	}

	modelType := param.Model

	//#todo need image param validation
	updatedPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(ctx, param, local, modelType.Fields, inputPayload, doc.Data, deltaUpdate)
	if err != nil {
		return nil, err
	}
	doc.Data = updatedPayload

	// update the meta
	doc.Meta.UpdatedAt = utility.GetCurrentTime()
	doc.Meta.LastModifiedBy.ID = param.UserID
	doc.Meta.LastModifiedBy.IsProjectUser = param.Role.IsProjectUser

	projectDriver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}

	err = projectDriver.UpdateDocumentOfProject(ctx, param, doc, false)
	if err != nil {
		return nil, err
	}

	if len(modelType.Connections) > 0 {
		if len(disconnects) > 0 {
			v, err := utility.ConnectDisconnectParamBuilder(cache.Project, param.DocumentID, disconnects, modelType)
			if err != nil {
				return nil, err
			}
			param.ConDisParam = v
			err = projectDriver.DisconnectBuilder(ctx, param)
			if err != nil {
				return nil, err
			}
		}

		if len(connections) > 0 {
			v, err := utility.ConnectDisconnectParamBuilder(cache.Project, param.DocumentID, connections, modelType)
			if err != nil {
				return nil, err
			}
			param.ConDisParam = v
			err = projectDriver.ConnectBuilder(ctx, param)
			if err != nil {
				return nil, err
			}
		}
	}

	// if hook has actions
	for _, h := range hooks {
		if utility.ArrayContains(h.Events, "update") {
			if h.URL != "" {
				go s.runWebHook("update", h, doc)
			}
			if len(h.LogicExecutions) > 0 {
				for _, t := range h.LogicExecutions {
					for _, f := range cache.Project.Schema.Functions {
						if f.Name == t {
							go s.triggerFunction(ctx, f, cache, h, map[string]interface{}{
								"payload":     doc,
								"connections": connections,
								"disconnects": disconnects,
							})
							break
						}
					}
				}
			}
		}
	}

	return doc, nil
}

func (s *GraphQLServer) createAndConnectDocument(ctx context.Context, cache *models.ApplicationCache, param *models.CommonSystemParams, hooks []*models.Webhook, payload map[string]interface{}, connections map[string]interface{}) (*types.DefaultDocumentStructure, error) {

	if param.Model == nil {
		return nil, errors.New("model is required for create operation")
	}

	if len(payload) == 0 {
		return nil, errors.New("payload is required")
	}

	_id := utility.NewID()
	doc := &types.DefaultDocumentStructure{
		Key:      _id,
		ID:       _id,
		Type:     param.Model.Name,
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
			Status: param.DocPublishStatus,
		},
	}

	input := param.ResolveParams.Args
	// local support
	local := "en"
	if val, ok := input["local"].(string); ok {
		local = val
	}

	//#todo need image param validation
	newPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(ctx, param, local, param.Model.Fields, payload, make(map[string]interface{}), false)
	if err != nil {
		return nil, err
	}
	doc.Data = newPayload

	// create a new doc
	projectDriver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}
	newDoc, err := projectDriver.AddDocumentToProject(ctx, param, doc)
	if err != nil {
		return nil, err
	}

	_doc := newDoc.(*types.DefaultDocumentStructure)

	// check connection first before creating
	if len(param.Model.Connections) > 0 && len(connections) > 0 {
		_param := s.NewParam(param)
		v, err := utility.ConnectDisconnectParamBuilder(cache.Project, _doc.ID, connections, _param.Model)
		if err != nil {
			return nil, err
		}
		_param.ConDisParam = v
		err = projectDriver.ConnectBuilder(ctx, _param)
		if err != nil {
			return nil, err
		}
	}

	// if hook has actions
	for _, h := range hooks {
		if utility.ArrayContains(h.Events, "create") {
			if h.URL != "" {
				go s.runWebHook("create", h, doc)
			}
			if len(h.LogicExecutions) > 0 {
				for _, t := range h.LogicExecutions {
					for _, f := range cache.Project.Schema.Functions {
						if f.Name == t {
							go s.triggerFunction(ctx, f, cache, h, map[string]interface{}{
								"payload":     doc,
								"connections": connections,
								//"disconnects": disconnects,
							})
							break
						}
					}
				}
			}
		}
	}

	return doc, nil
}

func (s *GraphQLServer) MutationResolverFn(p graphql.ResolveParams) (interface{}, error) {

	cache, ok := utility.LegacyApplicationCache(p.Context)
	if !ok || cache == nil {
		return nil, errors.New("graphql context: application cache missing")
	}

	ctx := p.Context

	param := s.NewParam(cache.Param)

	if param.Role == nil || param.Role.ID == "" {
		return "", errors.New("bad request. Reload the Page again")
	}

	fieldName := utility.ExtractResourceName(p.Info.FieldName)

	var modelType *models.ModelType
	for _, model := range cache.Project.Schema.Models {
		if utility.ModelIDMatchesGraphQLField(model.Name, fieldName) {
			modelType = model
		}
	}

	if modelType == nil {
		return nil, errors.New("model Type Could Not be Found")
	}

	param.Model = modelType
	param.ResolveParams = &p

	// p == "none" || p == "all" || p == "own" || p == "auth"
	permission, err := mutationPermissionForRole(param.Role, modelType.Name)
	if err != nil {
		return nil, err
	}

	var hooks []*models.Webhook
	// call in the webhook
	for _, hookID := range modelType.HookIds {
		hook, err := s.SystemDriver.GetWebHook(ctx, param.ProjectID, hookID)
		if err != nil {
			return nil, err
		}
		hooks = append(hooks, hook)
	}

	// fetch the doc again with query
	projectDriver, err := s.GraphQLExecutor.GetProjectDriver(ctx)
	if err != nil {
		return nil, err
	}

	action := utility.ExtractActionName(p.Info.FieldName)
	switch action {
	case "upsert":

		var payloads []map[string]interface{}
		if vals, ok := p.Args["payloads"].([]interface{}); ok && len(vals) > 0 {
			for _, val := range vals {
				payloads = append(payloads, val.(map[string]interface{}))
			}
		}

		var responses []*types.DefaultDocumentStructure
		// for each payload, create a new doc or update the existing doc
		var errs []string
		for _, payload := range payloads {
			if _id, ok := payload["_id"]; ok && (_id != nil && _id != "") {
				param.DocumentID = _id.(string)
				param.DocPublishStatus = "published"
				// update the existing doc
				// upsert has no delta update support for now
				_doc, err := s.updateAndConnectDocument(ctx, cache, param, hooks, payload, permission, nil, nil, false) //#TODO add connections support
				if err != nil {
					errs = append(errs, err.Error())
				}
				responses = append(responses, _doc)
			} else {

				// connection is only for create operation
				var connections map[string]interface{}
				if _connect, ok := payload["_connect"].(map[string]interface{}); ok && len(_connect) > 0 {
					connections = _connect
				}

				// create a new doc
				param.DocPublishStatus = "published"
				_doc, err := s.createAndConnectDocument(ctx, cache, param, hooks, payload, connections) //#TODO add connections support
				if err != nil {
					errs = append(errs, err.Error())
				}
				responses = append(responses, _doc)
			}
		}
		if len(errs) > 0 {
			return nil, errors.New(strings.Join(errs, " | "))
		}
		return responses, nil
	case "create":

		if !roleBypassesMutationACL(param.Role) {
			switch permission.Create {
			case "none":
				return nil, errors.New("creation is not permitted")
			case "auth":
				if !param.Role.IsProjectUser {
					return nil, errors.New("authentication is required to Create a Document")
				}
			}
		}

		if modelType.SinglePage && modelType.SinglePageUUID != "" {
			return nil, fmt.Errorf("document with an id :%s already exists. create operation is not permitted. use update instead", modelType.SinglePageUUID)
		}

		fields := p.Args

		if val, ok := fields["status"]; ok {
			param.DocPublishStatus = val.(string)
		} else {
			param.DocPublishStatus = "draft" // default is draft
		}

		var connections map[string]interface{}
		if val, ok := p.Args["connect"].(map[string]interface{}); ok && len(val) > 0 {
			connections = val
		}

		var payload map[string]interface{}
		if userInputPayload, ok := p.Args["payload"].(map[string]interface{}); ok && len(userInputPayload) > 0 {
			payload = userInputPayload
		}

		_doc, err := s.createAndConnectDocument(ctx, cache, param, hooks, payload, connections)
		if err != nil {
			return nil, err
		}

		// fetch the doc again with query
		param.DocumentID = _doc.ID
		createdDoc, err := projectDriver.GetSingleProjectDocument(ctx, param)
		if err != nil {
			return nil, err
		}

		return createdDoc, nil
	case "update":
		fields := p.Args
		if val, ok := fields["_id"].(string); ok && val != "" {
			param.DocumentID = val
		} else {
			return nil, errors.New("id is required for an update")
		}

		var connections map[string]interface{}
		if val, ok := p.Args["connect"].(map[string]interface{}); ok && len(val) > 0 {
			connections = val
		}

		var disconnects map[string]interface{}
		if val, ok := p.Args["disconnect"].(map[string]interface{}); ok && len(val) > 0 {
			disconnects = val
		}

		var payload map[string]interface{}
		if userInputPayload, ok := p.Args["payload"].(map[string]interface{}); ok && len(userInputPayload) > 0 {
			payload = userInputPayload
		}

		var deltaUpdate bool
		if val, ok := p.Args["deltaUpdate"].(bool); ok {
			deltaUpdate = val
		}

		_, err := s.updateAndConnectDocument(ctx, cache, param, hooks, payload, permission, connections, disconnects, deltaUpdate)
		if err != nil {
			return nil, err
		}

		doc, err := projectDriver.GetSingleProjectDocument(ctx, param)
		if err != nil {
			return nil, err
		}

		return doc, nil
	case "delete":
		fields := p.Args
		uids := fields["_ids"].([]interface{})
		var documentToDelete uint32
		for _, id := range uids {
			param.DocumentID = id.(string)
			doc, err := projectDriver.GetSingleProjectDocument(ctx, param)
			if err != nil {
				return nil, err
			}

			if !roleBypassesMutationACL(param.Role) {
				if permission == nil {
					return nil, errors.New("internal error: mutation permission is not resolved")
				}
				switch permission.Delete {
				case "none":
					return nil, errors.New("Update is not permitted")
				case "auth":
					if !param.Role.IsProjectUser {
						return nil, errors.New("Authentication is required to Update a Document")
					}
					break
				case "own":
					if doc.Type == "user" && param.Role.IsProjectUser && param.UserID != doc.ID {
						return nil, errors.New("You are not authorized to delete this document")
					} else if doc.Meta.CreatedBy.IsProjectUser && doc.Meta.CreatedBy.ID != param.UserID {
						return nil, errors.New("You are not authorized to delete this document")
					}
				}
			}

			err = projectDriver.DeleteDocumentFromProject(ctx, param)
			if err != nil {
				return nil, err
			}
			documentToDelete++
		}

		// if hook has actions
		for _, h := range hooks {
			if utility.ArrayContains(h.Events, "delete") {
				if h.URL != "" {
					go s.runWebHook("delete", h, uids)
				}
				if len(h.LogicExecutions) > 0 {
					for _, t := range h.LogicExecutions {
						for _, f := range cache.Project.Schema.Functions {
							if f.Name == t {
								go s.triggerFunction(ctx, f, cache, h, map[string]interface{}{
									"payload": uids,
									//"connections": connections,
									//"disconnects": disconnects,
								})
								break
							}
						}
					}
				}
			}
		}

		return map[string]interface{}{
			"response": "Documents has been Deleted along with its connections if there were any",
		}, nil
	default:
		return nil, errors.New("invalid Actions")
	}

}

package resolver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/tailor-platform/graphql"
)

// roleBypassesMutationACL is true for project admin (IsAdmin or id admin/owner).
func roleBypassesMutationACL(r *models.Role) bool {
	return utility.RoleBypassesDataACL(r)
}

// mutationPermissionForRole returns non-nil API CRUD scopes for the model via EffectivePermission.
func mutationPermissionForRole(role *models.Role, modelName string) (*models.APIPermission, error) {
	if role == nil {
		return nil, errors.New("role is required")
	}
	perm := utility.EffectivePermission(role, modelName)
	if perm == nil {
		return nil, errors.New("internal error: mutation permission is not resolved")
	}
	return perm, nil
}

// ownDocOwnerID keeps the resolver's historical call signature while sharing
// the same canonical ownership implementation used by non-SQL drivers.
func ownDocOwnerID(doc *types.DefaultDocumentStructure) (ownerID string, isUserModel bool) {
	return utility.OwnDocumentOwnerID(doc)
}

// validateRelationTargetOwnership verifies own-scoped relation targets before
// a public project-user mutation reaches a driver. The relation builder
// records the target model; this check resolves its persisted canonical owner
// instead of trusting a field name or caller-controlled metadata.
func (s *GraphQLServer) validateRelationTargetOwnership(
	ctx context.Context,
	driver interfaces.ProjectDBInterface,
	cache *models.ApplicationCache,
	root *models.CommonSystemParams,
	params []*models.ConnectDisconnectParam,
) error {
	if root == nil || root.Role == nil || !root.Role.IsProjectUser || roleBypassesMutationACL(root.Role) {
		return nil
	}
	if driver == nil || cache == nil || cache.Project == nil || cache.Project.Schema == nil {
		return utility.ErrRelationTargetForbidden
	}
	userID := strings.TrimSpace(root.UserID)
	if userID == "" {
		return utility.ErrRelationTargetForbidden
	}
	for _, relation := range params {
		if relation == nil {
			return utility.ErrRelationTargetForbidden
		}
		if !relation.TargetRequiresOwnership {
			continue
		}
		targetModel := resolveProjectModelFromSchema(cache.Project.Schema.Models, relation.TargetModelName)
		if targetModel == nil {
			return utility.ErrRelationTargetForbidden
		}
		for _, targetID := range relation.ActionIDs {
			targetID = strings.TrimSpace(targetID)
			if targetID == "" {
				return utility.ErrRelationTargetForbidden
			}
			lookup := s.NewParam(root)
			lookup.Model = targetModel
			lookup.DocumentID = targetID
			lookup.ProjectSchemaModels = cache.Project.Schema.Models
			lookup.IsSystemRequest = true
			raw, lookupErr := driver.GetSingleRawDocumentFromProject(ctx, lookup)
			target, found, err := existingDocumentFromLookup(raw, lookupErr)
			if err != nil || !found || target == nil {
				return utility.ErrRelationTargetForbidden
			}
			ownerID, _ := ownDocOwnerID(target)
			if strings.TrimSpace(ownerID) != userID {
				return utility.ErrRelationTargetForbidden
			}
		}
		relation.MarkTargetOwnershipVerified()
	}
	return nil
}

func (s *GraphQLServer) updateAndConnectDocument(ctx context.Context, cache *models.ApplicationCache, param *models.CommonSystemParams, hooks []*models.Webhook, userInputPayload map[string]interface{}, permission *models.APIPermission, connections, disconnects map[string]interface{}, deltaUpdate bool) (*types.DefaultDocumentStructure, error) {

	relationWork := len(connections) > 0 || len(disconnects) > 0
	if len(userInputPayload) == 0 && !relationWork {
		return nil, errors.New("payload is required")
	}

	dbCtx := models.ProjectDBContext(publicProjectDBContext(cache, ctx), param)
	driver, err := s.GraphQLExecutor.GetProjectDriver(dbCtx)
	if err != nil {
		return nil, err
	}

	raw, err := driver.GetSingleRawDocumentFromProject(dbCtx, param)
	if err != nil {
		return nil, err
	}
	doc := raw.(*types.DefaultDocumentStructure)

	ownerID, isUserModel := ownDocOwnerID(doc)
	if err := utility.AuthorizeModelUpdate(param.Role, param.Model.Name, ownerID, isUserModel, param.UserID); err != nil {
		return nil, err
	}
	if err := utility.ValidateProjectUserRelationEdges(param.Role, param.UserID, connections, disconnects); err != nil {
		return nil, err
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
	if inputPayload == nil {
		inputPayload = make(map[string]interface{})
	}

	if len(inputPayload) == 0 && !relationWork {
		return nil, errors.New("user input payload is required")
	}
	beforeData := make(map[string]interface{}, len(doc.Data))
	for key, value := range doc.Data {
		beforeData[key] = value
	}

	// local support
	local := "en"
	if val, ok := input["local"].(string); ok {
		local = val
	}

	modelType := param.Model
	var connectParams []*models.ConnectDisconnectParam
	var disconnectParams []*models.ConnectDisconnectParam
	if len(modelType.Connections) > 0 {
		if len(disconnects) > 0 {
			disconnectParams, err = utility.DisconnectDisconnectParamBuilderForRole(cache.Project, param.DocumentID, param.Role, disconnects, modelType)
			if err != nil {
				return nil, err
			}
			if err := s.validateRelationTargetOwnership(ctx, driver, cache, param, disconnectParams); err != nil {
				return nil, err
			}
			if err := utility.ValidateProjectUserRelationParams(param.Role, param.UserID, disconnectParams, true); err != nil {
				return nil, err
			}
		}
		if len(connections) > 0 {
			connectParams, err = utility.ConnectDisconnectParamBuilderForRole(cache.Project, param.DocumentID, param.Role, connections, modelType)
			if err != nil {
				return nil, err
			}
			if err := s.validateRelationTargetOwnership(ctx, driver, cache, param, connectParams); err != nil {
				return nil, err
			}
			if err := utility.ValidateProjectUserRelationParams(param.Role, param.UserID, connectParams, false); err != nil {
				return nil, err
			}
		}
	}

	//#todo need image param validation
	if len(inputPayload) > 0 {
		updatedPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(ctx, param, local, modelType.Fields, inputPayload, doc.Data, deltaUpdate)
		if err != nil {
			return nil, err
		}
		if err := utility.ValidateOwnerFieldMutation(param.Role, param.UserID, beforeData, updatedPayload); err != nil {
			return nil, err
		}
		doc.Data = updatedPayload
	}

	// update the meta
	if doc.Meta == nil {
		doc.Meta = &types.MetaField{}
	}
	doc.Meta.UpdatedAt = utility.GetCurrentTime()
	if doc.Meta.LastModifiedBy == nil {
		doc.Meta.LastModifiedBy = &types.SystemUser{}
	}
	doc.Meta.LastModifiedBy.ID = param.UserID
	doc.Meta.LastModifiedBy.IsProjectUser = param.Role.IsProjectUser

	projectDriver, err := s.GraphQLExecutor.GetProjectDriver(dbCtx)
	if err != nil {
		return nil, err
	}

	err = projectDriver.UpdateDocumentOfProject(dbCtx, param, doc, false)
	if err != nil {
		return nil, err
	}

	if len(modelType.Connections) > 0 {
		if len(disconnectParams) > 0 {
			param.ConDisParam = disconnectParams
			err = projectDriver.DisconnectBuilder(dbCtx, param)
			if err != nil {
				return nil, err
			}
		}

		if len(connectParams) > 0 {
			param.ConDisParam = connectParams
			err = projectDriver.ConnectBuilder(dbCtx, param)
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
	if err := utility.ValidateProjectUserRelationEdges(param.Role, param.UserID, connections, nil); err != nil {
		return nil, err
	}

	dbCtx := models.ProjectDBContext(publicProjectDBContext(cache, ctx), param)

	_id := utility.NewID()
	doc := &types.DefaultDocumentStructure{
		Key:  _id,
		ID:   _id,
		Type: param.Model.Name,
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

	if err := utility.ValidateOwnerFieldOnCreate(param.Role, param.UserID, payload); err != nil {
		return nil, err
	}

	//#todo need image param validation
	newPayload, err := s.GraphQLExecutor.HandlePayloadFormatting(ctx, param, local, param.Model.Fields, payload, make(map[string]interface{}), false)
	if err != nil {
		return nil, err
	}
	if err := utility.ValidateOwnerFieldOnCreate(param.Role, param.UserID, newPayload); err != nil {
		return nil, err
	}
	doc.Data = newPayload

	// create a new doc
	projectDriver, err := s.GraphQLExecutor.GetProjectDriver(dbCtx)
	if err != nil {
		return nil, err
	}
	var connectParams []*models.ConnectDisconnectParam
	if len(param.Model.Connections) > 0 && len(connections) > 0 {
		connectParams, err = utility.ConnectDisconnectParamBuilderForRole(cache.Project, _id, param.Role, connections, param.Model)
		if err != nil {
			return nil, err
		}
		if err := s.validateRelationTargetOwnership(ctx, projectDriver, cache, param, connectParams); err != nil {
			return nil, err
		}
		if err := utility.ValidateProjectUserRelationParams(param.Role, param.UserID, connectParams, false); err != nil {
			return nil, err
		}
	}
	newDoc, err := projectDriver.AddDocumentToProject(dbCtx, param, doc)
	if err != nil {
		return nil, err
	}

	_doc := newDoc.(*types.DefaultDocumentStructure)

	if len(connectParams) > 0 {
		_param := s.NewParam(param)
		_param.DocumentID = _doc.ID
		_param.ConDisParam = connectParams
		err = projectDriver.ConnectBuilder(dbCtx, _param)
		if err != nil {
			_param.DocumentID = _doc.ID
			_ = projectDriver.DeleteDocumentFromProject(dbCtx, _param)
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

func (s *GraphQLServer) MutationResolverFn(p graphql.ResolveParams) (resp interface{}, err error) {
	cache, ok := utility.LegacyApplicationCache(p.Context)
	if !ok || cache == nil {
		return nil, errors.New("graphql context: application cache missing")
	}

	defer func() {
		if err != nil && cache != nil && cache.Project != nil {
			s.NotifyEventSinkPlugins(p.Context, cache.Project.ID, "error", map[string]interface{}{
				"source": "mutation",
				"field":  p.Info.FieldName,
				"error":  err.Error(),
			})
		}
	}()

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
	// Required for SQLite EnsureRelationArtifactsFromSchema on connect/disconnect.
	// Public queries already set this; system upsertModelData does too. Without it,
	// maybeEnsureRelationDDLForMutation no-ops and FK connects fail with
	// "no such column: <model>_id" on tenants whose tables predate the relation.
	if cache.Project != nil && cache.Project.Schema != nil {
		param.ProjectSchemaModels = cache.Project.Schema.Models
	}

	dbCtx := models.ProjectDBContext(publicProjectDBContext(cache, ctx), param)

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
	projectDriver, err := s.GraphQLExecutor.GetProjectDriver(dbCtx)
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
				} else if _doc != nil {
					s.EmitModelChange(dbCtx, param.ProjectID, modelType.Name, models.ChangeEventUpdated, _doc.ID, _doc)
				}
				responses = append(responses, _doc)
			} else {
				if err := utility.AuthorizeModelCreate(param.Role, modelType.Name); err != nil {
					errs = append(errs, err.Error())
					responses = append(responses, nil)
					continue
				}
				if err := s.enforcePlanCreateQuota(dbCtx, cache, param, modelType.Name); err != nil {
					errs = append(errs, err.Error())
					responses = append(responses, nil)
					continue
				}
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
				} else if _doc != nil {
					s.EmitModelChange(dbCtx, param.ProjectID, modelType.Name, models.ChangeEventCreated, _doc.ID, _doc)
				}
				responses = append(responses, _doc)
			}
		}
		if len(errs) > 0 {
			return nil, errors.New(strings.Join(errs, " | "))
		}
		return responses, nil
	case "create":

		if err := utility.AuthorizeModelCreate(param.Role, modelType.Name); err != nil {
			return nil, err
		}
		if err := s.enforcePlanCreateQuota(dbCtx, cache, param, modelType.Name); err != nil {
			return nil, err
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
		createdDoc, err := projectDriver.GetSingleProjectDocument(dbCtx, param)
		if err != nil {
			return nil, err
		}

		s.EmitModelChange(dbCtx, param.ProjectID, modelType.Name, models.ChangeEventCreated, _doc.ID, createdDoc)
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
		if userInputPayload, ok := p.Args["payload"].(map[string]interface{}); ok {
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

		doc, err := projectDriver.GetSingleProjectDocument(dbCtx, param)
		if err != nil {
			return nil, err
		}

		s.EmitModelChange(dbCtx, param.ProjectID, modelType.Name, models.ChangeEventUpdated, param.DocumentID, doc)
		return doc, nil
	case "delete":
		fields := p.Args
		uids := fields["_ids"].([]interface{})
		var documentToDelete uint32
		for _, id := range uids {
			param.DocumentID = id.(string)
			doc, err := projectDriver.GetSingleProjectDocument(dbCtx, param)
			if err != nil {
				return nil, err
			}

			ownerID, isUserModel := ownDocOwnerID(doc)
			if err := utility.AuthorizeModelDelete(param.Role, modelType.Name, ownerID, isUserModel, param.UserID); err != nil {
				return nil, err
			}

			err = projectDriver.DeleteDocumentFromProject(dbCtx, param)
			if err != nil {
				return nil, err
			}
			s.EmitModelChange(dbCtx, param.ProjectID, modelType.Name, models.ChangeEventDeleted, param.DocumentID, nil)
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

package resolver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/functions"
	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	hcplugin "github.com/hashicorp/go-plugin"
	"github.com/tailor-platform/graphql"
)

func (s *GraphQLServer) HandleApitoFunction(ctx context.Context, cache *models.ApplicationCache, fnName string, args map[string]interface{}) (interface{}, *models.ApitoFunction, error) {

	param := s.NewParam(cache.Param)
	project := cache.Project
	if project == nil || project.Schema == nil {
		return nil, nil, errors.New("function Not Found, Something is Wrong")
	}

	var _function *models.ApitoFunction
	for _, f := range project.Schema.Functions {
		if f == nil || f.Name != fnName {
			continue
		}
		_function = f
		break
	}

	if _function == nil {
		return nil, nil, errors.New("function Not Found, Something is Wrong")
	}

	var _payload interface{}
	reqModel := ""
	if _function.Request != nil {
		reqModel = _function.Request.Model
	}

	switch reqModel {
	case "JSON", "":
		if val, ok := args["payload"].(map[string]interface{}); ok && len(val) > 0 {
			_payload = val
		} else if _function.Request != nil && _function.Request.OptionalPayload {
			_payload = map[string]interface{}{}
		} else if reqModel == "" {
			_payload = map[string]interface{}{}
		} else {
			return nil, nil, errors.New("payload is required")
		}
		if param.UserID != "" {
			if m, ok := _payload.(map[string]interface{}); ok {
				m["user_id"] = param.UserID
			}
		}
	default:
		doc, ok := args["payload"].(*types.DefaultDocumentStructure)
		if !ok {
			// Allow map payloads for non-JSON models when callers pass raw maps
			if val, ok := args["payload"].(map[string]interface{}); ok {
				_payload = val
			} else {
				return nil, nil, errors.New("invalid payload type")
			}
		} else {
			_payload = doc
			if param.UserID != "" {
				doc.Data["user_id"] = param.UserID
			}
		}
	}

	runtime := _function.EffectiveRuntime()

	// Apito Functions platform (deno / wasm)
	if _function.IsApitoFunctionsRuntime() {
		if s.FunctionRuntime == nil {
			return nil, nil, fmt.Errorf("function runtime manager not configured for runtime %q", runtime)
		}
		reqMap, _ := _payload.(map[string]interface{})
		if reqMap == nil {
			reqMap = map[string]interface{}{"payload": _payload}
		}
		tenantID, err := s.applyFunctionTenantScope(ctx, cache, models.FunctionTenantScopeLive, "")
		if err != nil {
			return nil, nil, err
		}
		role := ""
		if param != nil && param.Role != nil {
			role = param.Role.ID
		}
		env := functions.BuildEnvelope(_function, project.ID, tenantID, param.UserID, role, reqMap, utility.NewID())
		// Live execution uses active revision artifact when present; draft Source is for edit/test.
		if s.FunctionRuntime != nil {
			env.Source = functions.ResolveActiveSource(ctx, s.FunctionRuntime.Artifacts(), _function)
		}
		if env.Source == "" {
			env.Source = _function.Source
		}
		if err := s.registerFunctionInvocation(ctx, cache, env); err != nil {
			return nil, nil, err
		}
		defer functions.GlobalInvocationRegistry.Unregister(env.InvocationID)
		result, err := s.FunctionRuntime.Invoke(ctx, env)
		if err != nil {
			return nil, nil, err
		}
		if result == nil || !result.OK {
			msg := "function execution failed"
			if result != nil && result.Error != "" {
				msg = result.Error
			}
			return nil, nil, errors.New(msg)
		}
		return result.Response, _function, nil
	}

	// Legacy HashiCorp path
	if strings.HasPrefix(_function.FunctionProviderID, "hc-") || runtime == models.FunctionRuntimeHashicorp {
		if _function.FunctionProviderID == "" {
			return nil, nil, fmt.Errorf("hashicorp function missing function_provider_id")
		}
		var _plugin *hcplugin.Client
		if val, ok := s.HashiCorpPluginCache[_function.FunctionProviderID]; ok && val != nil {
			_plugin = val.Client
		} else {
			return nil, nil, fmt.Errorf("%s plugin Not loaded, reinstall the plugin", _function.FunctionProviderID)
		}

		rpcClient, err := _plugin.Client()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get RPC client for HashiCorp plugin: %v", err)
		}
		raw, err := rpcClient.Dispense(_const.FunctionPluginRPCName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to dispense HashiCorp function plugin: %v", err)
		}
		functionPlugin, ok := raw.(interfaces.HashiCorpPluginInterface)
		if !ok {
			return nil, nil, fmt.Errorf("HashiCorp plugin does not implement FunctionPluginInterface")
		}

		ctx = context.WithValue(ctx, "project_schema", project.Schema)
		ctx = context.WithValue(ctx, "project_id", project.ID)
		result, err := functionPlugin.Execute(ctx, _payload)
		if err != nil {
			return nil, nil, err
		}
		return result, _function, nil
	}

	return nil, nil, fmt.Errorf("unsupported function runtime %q for %s (set runtime_config.runtime to deno, wasm, or use hc-* provider)", runtime, _function.Name)
}

func (s *GraphQLServer) ApitoFunctionResolverFn(p graphql.ResolveParams) (interface{}, error) {

	cache, ok := utility.LegacyApplicationCache(p.Context)
	if !ok || cache == nil {
		return nil, errors.New("graphql context: application cache missing")
	}

	if s.Cfg != nil && s.Cfg.RoleAgnosticSchemaCache && cache.Param != nil && cache.Param.Role != nil {
		r := cache.Param.Role
		if !r.IsAdmin && !utility.ArrayContains(r.LogicExecutions, p.Info.FieldName) {
			return nil, errors.New("permission denied: function not allowed for this role")
		}
	}

	resp, _fn, err := s.HandleApitoFunction(p.Context, cache, p.Info.FieldName, p.Args)
	if err != nil {
		return nil, err
	}

	switch cache.GraphqlRequest.QueryType {
	case "mutation":
		switch _fn.Response.Model {
		case "JSON":
			return map[string]interface{}{
				"JSON": resp,
			}, err
		default:
			return resp, err
		}
	default:
		switch _fn.Response.Model {
		case "JSON":
			return map[string]interface{}{
				"JSON": resp,
			}, err
		default:
			if _fn.Response.IsArray {
				return resp, err
			}
			return map[string]interface{}{
				"data": map[string]interface{}{
					_fn.Name: resp,
				},
			}, err
		}
	}
}

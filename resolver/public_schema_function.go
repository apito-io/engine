package resolver

import (
	"context"
	"errors"
	"fmt"
	"strings"

	_const "github.com/apito-io/engine/const"
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

	var _function *models.ApitoFunction
	for _, f := range project.Schema.Functions {
		if f.Name == fnName && f.FunctionProviderID != "" {
			_function = f
			break
		}
	}

	if _function == nil {
		return nil, nil, errors.New("function Not Found, Something is Wrong")
	}

	var _payload interface{}

	switch _function.Request.Model {
	case "JSON":
		if val, ok := args["payload"].(map[string]interface{}); ok && len(val) > 0 {
			_payload = val
		} else {
			return nil, nil, errors.New("payload is required")
		}
		// inject user id if available
		if param.UserID != "" {
			_payload.(map[string]interface{})["user_id"] = param.UserID
		}
	default:
		doc, ok := args["payload"].(*types.DefaultDocumentStructure)
		if !ok {
			return nil, nil, errors.New("invalid payload type")
		}
		_payload = doc

		// inject user id if available
		if param.UserID != "" {
			doc.Data["user_id"] = param.UserID
		}
	}

	if strings.HasPrefix(_function.FunctionProviderID, "hc-") {

		var _plugin *hcplugin.Client

		// HashiCorp plugin
		if val, ok := s.HashiCorpPluginCache[_function.FunctionProviderID]; ok && val != nil {
			_plugin = val.Client
			//_configuration = val.PluginConfigurations
		} else {
			return nil, nil, fmt.Errorf("%s plugin Not loaded, reinstall the plugin", _function.FunctionProviderID)
		}

		var result interface{}

		// Get the RPC client first
		rpcClient, err := _plugin.Client()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get RPC client for HashiCorp plugin: %v", err)
		}

		// Get the function plugin RPC client
		raw, err := rpcClient.Dispense(_const.FunctionPluginRPCName)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to dispense HashiCorp function plugin: %v", err)
		}

		functionPlugin, ok := raw.(interfaces.HashiCorpPluginInterface)
		if !ok {
			return nil, nil, fmt.Errorf("HashiCorp plugin does not implement FunctionPluginInterface")
		}

		fmt.Println(fmt.Sprintf(`------ Loading %s HashiCorp Function Plugin -------`, _function.Name))

		// inject schema in the context
		ctx = context.WithValue(ctx, "project_schema", project.Schema)
		// inject project id to context value
		ctx = context.WithValue(ctx, "project_id", project.ID)

		// For HashiCorp plugins, for now we skip injectable services injection
		// to avoid serialization issues. This will be implemented properly with gRPC later
		result, err = functionPlugin.Execute(ctx, _payload)
		if err != nil {
			return nil, nil, err
		}

		return result, _function, nil

	} else {

		// Local plugin function execution removed - use HashiCorp plugins instead
		return nil, nil, fmt.Errorf("Local plugin function provider %s no longer supported. Use HashiCorp plugins instead", _function.FunctionProviderID)
	}
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
			if _fn.Response.IsArray {
				return resp, err
			} else {
				return resp, err
			}
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
			} else {
				return map[string]interface{}{
					"data": map[string]interface{}{
						_fn.Name: resp,
					},
				}, err
			}
		}
	}
}

/*
func (s *GraphQLServer) ApitoFunctionResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var cred *protobuff.ThirdPartyCredential
	if val, ok := s.PluginConfigurations["aws"]; ok {
		cred = val.Credentials
	} else {
		return nil, errors.New("AWS Credentials are not Set")
	}

	for _, f := range s.ProjectRawSchemas.Functions {
		if f.Name == p.Info.FieldName && f.FunctionConnected {

			sess, err := session.NewSession(&aws.Config{
				Region:      aws.String(f.ProviderConfig.Region),
				Credentials: credentials.NewStaticCredentials(cred.AccessKey, cred.SecretKey, ""),
			})
			if err != nil {
				return nil, err
			}
			_, err = sess.Config.Credentials.Get()
			if err != nil {
				return nil, err
			}

			var data = make(map[string]interface{})
			switch f.Request.Model {
			case "JSON":
				for k, v := range p.Args["payload"].(map[string]interface{}) {
					data[k] = v
				}
				break
			default:
				data = p.Args["payload"].(map[string]interface{})
			}

			if len(data) == 0 {
				return nil, errors.New("No Request Payload is Found")
			}

			payload, err := json.Marshal(map[string]interface{}{
				"payload": data, // user request payload
				"meta": map[string]interface{}{
					"user_id": s.Param.UserId,
					"role":    s.Param.Role,
				},
			})

			svc := lambda.New(sess)
			input := &lambda.InvokeInput{
				FunctionName:   aws.String(f.ProviderConfig.RemoteFunctionName),
				Payload:        payload,
				InvocationType: aws.String("RequestResponse"),
				LogType:        aws.String("Tail"),
				//Qualifier:      aws.String("current"),
			}

			invokeResponse, err := svc.Invoke(input)
			if err != nil {
				return nil, err
			}

			var result map[string]interface{}
			err = json.Unmarshal(invokeResponse.Payload, &result)
			if err != nil {
				return nil, err
			}

			if err, ok := result["errorMessage"].(string); ok {
				return nil, errors.New("Lambda Execution Error : " + err)
			}

			switch f.Response.Model {
			case "JSON":
				return map[string]interface{}{
					"JSON": result,
				}, nil
			default:
				return result, nil
			}
		}
	}

	return map[string]interface{}{
		"JSON": map[string]interface{}{
			"msg": "Function Not Connected to any Cloud Provider",
		},
	}, nil
}*/

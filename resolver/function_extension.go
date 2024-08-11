package resolver

import (
	"errors"
	"fmt"
	"plugin"

	"github.com/apito-io/buffers/interfaces"
	"github.com/apito-io/buffers/protobuff"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
)

func (s *GraphQLServer) ApitoFunctionResolverFn(p graphql.ResolveParams) (interface{}, error) {

	var (
		v      = p.Context.Value
		router = v("router").(echo.Context)
	)

	cache, err := s.GetApplicationCache(router)
	if err != nil {
		return nil, err
	}

	var _function *protobuff.CloudFunction
	for _, f := range cache.Project.Schema.Functions {
		if f.Name == p.Info.FieldName && f.FunctionConnected {
			_function = f
			break
		}
	}

	var _payload = make(map[string]interface{})
	switch _function.Request.Model {
	case "JSON":
		for k, v := range p.Args["payload"].(map[string]interface{}) {
			_payload[k] = v
		}
		break
	default:
		_payload = p.Args["payload"].(map[string]interface{})
	}

	if len(_payload) == 0 {
		return nil, errors.New("no Request Payload is Found")
	}

	var _plugin *plugin.Plugin
	var _configuration *protobuff.PluginDetails
	if val, ok := s.LocalPluginCache[_function.FunctionProviderId]; ok {
		_plugin = val.Plugin
		_configuration = val.PluginConfigurations
	}

	var result interface{}

	// 2. look up a symbol (an exported function or variable)
	// in this case, variable Greeter
	providerPluginLookUp, err := _plugin.Lookup(_function.ProviderExportedVariable)
	if err != nil {
		return nil, err
	}

	var providerLoadedPlugin interfaces.FunctionPluginInterface
	providerLoadedPlugin, ok := providerPluginLookUp.(interfaces.FunctionPluginInterface)
	if !ok {
		return nil, errors.New(fmt.Sprintf(`%s plugin load failed`, _function.Name))
	}

	fmt.Println(fmt.Sprintf(`------ Loading %s Function Plugin -------`, _function.Name))

	result, err = providerLoadedPlugin.Execute(map[string]interface{}{
		"payload": _payload,
		"conf":    _configuration,
	})
	if err != nil {
		return nil, err
	}

	switch _function.Response.Model {
	case "JSON":
		return map[string]interface{}{
			"JSON": result,
		}, err
	default:
		return result, err
	}
}

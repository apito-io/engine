package models

import (
	"plugin"

	"github.com/apito-io/buffers/plugins"
	"github.com/apito-io/buffers/protobuff"
)

type PluginCache struct {
	Plugin               *plugin.Plugin
	PluginConfigurations *protobuff.PluginDetails
}

type FunctionCache struct {
	Functions         *plugin.Plugin
	FuncConfiguration *protobuff.CloudFunction
}

/*type LoadedPlugin struct {
	// application specific plugin & function
	LocalPluginCache   map[string]*LocalPluginCache
	FunctionCache map[string]*FunctionCache
}*/

type LoadedPluginCache struct {
	//Plugins []*LoadedPlugin                      `json:"plugins,omitempty"`
	Schemas *plugins.ThirdPartyGraphQLSchemas `json:"schemas,omitempty"`
	Routes  []*plugins.ThirdPartyRESTApi      `json:"routes,omitempty"`
}

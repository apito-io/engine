package models

import (
	"plugin"

	"github.com/apito-io/types/protobuff"
	hcplugin "github.com/hashicorp/go-plugin"
)

// System apito function model ( old )
type FunctionCache struct {
	Functions         *plugin.Plugin
	FuncConfiguration *ApitoFunction
}

// new plugin system models
type PluginCache struct {
	Plugin               *plugin.Plugin
	PluginConfigurations *protobuff.PluginDetails
}

// HashiCorpPluginCache for HashiCorp go-plugin system
type HashiCorpPluginCache struct {
	Client               *hcplugin.Client
	PluginConfigurations *protobuff.PluginDetails
	RPCClient            hcplugin.ClientProtocol
}

/*type LoadedPlugin struct {
	// application specific plugin & function
	LocalPluginCache   map[string]*LocalPluginCache
	FunctionCache map[string]*FunctionCache
}*/

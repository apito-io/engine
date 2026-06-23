//go:build cloudflare

package models

import "github.com/apito-io/types/protobuff"

// HashiCorpPluginCache is a placeholder on Workers (no go-plugin).
type HashiCorpPluginCache struct {
	PluginConfigurations *protobuff.PluginDetails
}

// PluginCache placeholder for Workers builds.
type PluginCache struct {
	PluginConfigurations *protobuff.PluginDetails
}

// FunctionCache placeholder for Workers builds.
type FunctionCache struct {
	FuncConfiguration *ApitoFunction
}

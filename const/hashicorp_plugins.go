package _const

// Default handshake configuration for HashiCorp plugins
const (
	DefaultProtocolVersion  = 1
	DefaultMagicCookieKey   = "APITO_PLUGIN"
	DefaultMagicCookieValue = "apito_plugin_magic_cookie_v1"
)

// Plugin types for HashiCorp system
const (
	HashiCorpNormalPlugin   = "normal"
	HashiCorpStoragePlugin  = "storage"
	HashiCorpFunctionPlugin = "function"
)

// RPC plugin names
const (
	NormalPluginRPCName    = "NormalPlugin"
	StoragePluginRPCName   = "StoragePlugin"
	FunctionPluginRPCName  = "FunctionPlugin"
	InjectedServiceRPCName = "InjectedService"
)

# HashiCorp Plugin Communication Protocol

This directory contains HashiCorp plugins that communicate with the Apito engine via RPC. This system provides better isolation and stability compared to the Go built-in plugin system.

## Plugin System Architecture

The Apito engine now supports **two parallel plugin systems**:

1. **Go Built-in Plugins** (`plugins/local/`) - Traditional `.so` files
2. **HashiCorp Plugins** (`plugins/hashicorp/`) - Separate process via RPC

## Communication Protocol

### Plugin Types

All plugin types support the following communication features:

- **InjectedDBOperationInterface**: Database operations, audit logging, debug services
- **Environment Variables**: Configuration and credentials
- **Context Management**: Project ID, tenant ID, user ID, request ID
- **Health Checks**: Plugin status monitoring
- **Metrics Collection**: Performance and usage tracking

### Available Plugin Interfaces

1. **HashiCorpNormalPluginInterface**

   - `Init(ctx, envVars) error`
   - `GetVersion(ctx) (string, error)`
   - `Execute(ctx, input) (interface{}, error)`

2. **HashiCorpStoragePluginInterface**

   - `Init(ctx, envVars) error`
   - `GetVersion(ctx) (string, error)`
   - `UploadFile(ctx, details) (interface{}, error)`
   - `RemoveFile(ctx, details) error`

3. **HashiCorpFunctionPluginInterface**
   - `Init(ctx, envVars) error`
   - `GetVersion(ctx) (string, error)`
   - `Exec(ctx, params) (interface{}, error)`

### Injected Services

All HashiCorp plugins have access to the following services via RPC:

- `GenerateTenantToken(ctx, token, tenantID) (string, error)`
- `GetProjectDetails(ctx, projectID) (*protobuff.Project, error)`
- `GetSingleResource(ctx, model, id, singlePageData) (interface{}, error)`
- `SearchResources(ctx, model, filter, aggregate) (interface{}, error)`
- `CreateNewResource(ctx, model, data, connection) (interface{}, error)`
- `UpdateResource(ctx, model, id, singlePageData, data, connect, disconnect) (interface{}, error)`
- `DeleteResource(ctx, model, id) error`
- `SendAuditLog(ctx, auditData) error`
- `Debug(ctx, stage, ...data) (interface{}, error)`

## Plugin Registration

Plugins are registered in `engine/plugins/hashicorp_plugin_list.go`:

```go
func LoadHashiCorpPluginRegistry() (map[string]*protobuff.PluginDetails, error) {
    return map[string]*protobuff.PluginDetails{
        "plugin-name": {
            ID:           "plugin-name",
            Type:         protobuff.PluginType_NORMAL,
            System:       protobuff.PluginSystem_HashiCorp,
            BinaryPath:   "plugin-name",
            HandshakeConfig: &protobuff.HashiCorpHandshakeConfig{
                ProtocolVersion:  _const.DefaultProtocolVersion,
                MagicCookieKey:   _const.DefaultMagicCookieKey,
                MagicCookieValue: _const.DefaultMagicCookieValue,
            },
        },
    }, nil
}
```

## Plugin Development

### Handshake Configuration

All plugins must use the standard handshake:

```go
var handshakeConfig = hcplugin.HandshakeConfig{
    ProtocolVersion:  1,
    MagicCookieKey:   "APITO_PLUGIN",
    MagicCookieValue: "apito_plugin_magic_cookie_v1",
}
```

### Plugin Map

Plugins should include both their main functionality and injected services:

```go
var pluginMap = map[string]hcplugin.Plugin{
    "NormalPlugin":     &plugins.HashiCorpNormalPlugin{},
    "InjectedService":  &plugins.InjectedDBOperationPlugin{},
}
```

## Deployment

1. Build your plugin binary
2. Place it in `plugins/hashicorp/your-plugin-name/your-plugin-name`
3. Register it in the plugin registry
4. Restart the Apito engine

## Benefits

- **Process Isolation**: Plugins run in separate processes
- **Language Agnostic**: Can be written in any language
- **Better Stability**: Plugin crashes don't affect the main process
- **Resource Management**: Better control over plugin resources
- **Security**: Natural process boundaries provide security isolation

## Plugin Lifecycle

1. **Discovery**: Engine scans `plugins/hashicorp/` directory
2. **Registration**: Loads plugin definitions from registry
3. **Initialization**: Starts plugin process via RPC
4. **Health Monitoring**: Continuous health checks
5. **Cleanup**: Graceful shutdown on engine stop

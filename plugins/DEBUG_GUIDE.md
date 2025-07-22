# HashiCorp Plugin Debugging Guide

This guide explains how to debug HashiCorp plugins using the VS Code configurations.

## Quick Start

### 1. Engine + All Plugins Debugging

Use the **"Engine Debug with HashiCorp Plugins"** launch configuration:

- Builds all plugins with debug flags
- Launches the engine with debugging enabled
- Sets `PLUGIN_DEBUG_MODE=true`

### 2. Individual Plugin Debugging

Choose from these launch configurations:

- **"Debug Protiva Plugin (Standalone)"** - Debug the Protiva function plugin
- **"Debug Hello World Plugin (Standalone)"** - Debug the hello world plugin
- **"Debug CDN Upload Plugin (Standalone)"** - Debug the CDN upload plugin

## Debugging Methods

### Method 1: Engine + Plugin Debugging (Recommended)

1. Set breakpoints in your plugin code
2. Select **"Engine Debug with HashiCorp Plugins"** from VS Code debug panel
3. Press F5 to start debugging
4. The engine will start and load plugins with debug symbols
5. Trigger plugin execution through GraphQL/REST API calls
6. Breakpoints in plugin code will be hit

### Method 2: Standalone Plugin Debugging

1. Set breakpoints in plugin code
2. Select the appropriate standalone plugin debug configuration
3. Press F5 to launch the plugin directly
4. This runs the plugin in isolation for testing

### Method 3: Attach to Running Plugin Process

For debugging plugins that are already running:

1. Start the engine with plugins
2. Find the plugin process ID:
   ```bash
   ps aux | grep hc-protiva-function-plugin | grep -v grep
   ```
3. Use **"Attach to Plugin Process"** configuration
4. Enter the process ID when prompted
5. VS Code will attach to the running plugin

## Available VS Code Tasks

Run these from the Command Palette (Ctrl+Shift+P) → "Tasks: Run Task":

### Build Tasks:

- **"Build All Plugins (Debug)"** - Build all plugins with debug flags
- **"Build HashiCorp Plugin - Protiva (Debug)"** - Build just Protiva plugin with debug flags
- **"Build HashiCorp Plugin - Hello World (Debug)"** - Build just Hello World plugin with debug flags
- **"Build HashiCorp Plugin - CDN Upload (Debug)"** - Build just CDN Upload plugin with debug flags

### Utility Tasks:

- **"Find Plugin Process PID"** - Find running plugin process IDs
- **"Run Engine with Debug Plugins"** - Run engine with debug plugins
- **"Check HashiCorp Plugin Status"** - Check which plugins are built

## Makefile Targets

From the `plugins/hashicorp/` directory:

```bash
# Build all plugins with debug flags
make build-debug-all

# Build regular plugins
make build-local-all

# Check plugin status
make status

# Clean all binaries
make clean-all

# Show help
make help
```

## Debug Binary Naming Convention

Debug binaries are suffixed with `-debug`:

- `hc-protiva-function-plugin-debug`
- `hc-hello-world-plugin-debug`
- `hc-apito-cdn-file-upload-debug`

## Environment Variables for Debugging

The debug configurations set:

- `PLUGIN_DEBUG_MODE=true` - Enables plugin debug mode
- Standard engine environment variables for local development

## Debugging Tips

### 1. Plugin Lifecycle Debugging

Set breakpoints in these plugin methods:

- `Init()` - Plugin initialization
- `Execute()` - Function execution (for function plugins)
- `UploadFile()` - File operations (for storage plugins)

### 2. RPC Communication Debugging

Set breakpoints in:

- `engine/plugins/hashicorp_plugin_rpc.go` - RPC client/server code
- Plugin RPC method implementations

### 3. Plugin Loading Debugging

Set breakpoints in:

- `engine/resolver/plugin_loader.go` - Plugin loading logic
- `engine/resolver/public_schema_function.go` - Function plugin execution

### 4. Log Output

Enable structured logging in plugins:

```go
p.logger.Debug("Debug message", "key", value)
p.logger.Info("Info message", "data", data)
p.logger.Error("Error occurred", "error", err)
```

## Common Debugging Scenarios

### 1. Plugin Won't Load

- Check plugin binary exists and has correct permissions
- Verify plugin is registered in `hashicorp_plugin_list.go`
- Check handshake configuration matches

### 2. RPC Communication Issues

- Verify gob type registration for custom types
- Check method signatures match interface definitions
- Ensure proper error handling in RPC methods

### 3. Function Plugin Issues

- Set breakpoints in `Execute()` method
- Check function name routing in switch statement
- Verify payload parsing and response formatting

## Example Debugging Session

1. Set a breakpoint in `hc-protiva-function-plugin/main.go` in the `login()` function
2. Launch "Engine Debug with HashiCorp Plugins"
3. Make a GraphQL query that calls the login function:
   ```graphql
   mutation {
     callFunction(
       function: "login"
       payload: { email: "test@example.com", password: "password123" }
     ) {
       data
     }
   }
   ```
4. The debugger will stop at your breakpoint
5. Step through the code to debug the login logic

## Troubleshooting

### Plugin Process Not Found

If you can't find the plugin process to attach to:

```bash
# List all go processes
ps aux | grep go

# Look for specific plugin
ps aux | grep hc-protiva-function-plugin
```

### Debug Symbols Missing

Ensure plugins are built with debug flags:

```bash
go build -gcflags="all=-N -l" -o plugin-debug .
```

### VS Code Not Stopping at Breakpoints

- Ensure you're using the debug binary, not the regular binary
- Check that the source file paths match
- Verify the plugin is actually being executed

## Additional Resources

- [VS Code Go Debugging](https://code.visualstudio.com/docs/languages/go#_debugging)
- [HashiCorp go-plugin Documentation](https://github.com/hashicorp/go-plugin)
- [Delve Debugger Documentation](https://github.com/go-delve/delve)

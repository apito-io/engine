# HashiCorp Plugin Debug Guide

## Overview

This guide explains how to debug HashiCorp plugins in the Apito Engine using the runDebug.sh approach.

## Debug Setup

### 1. Plugin Configuration

The plugin must be configured with `debug: true` in `plugins.yml`:

```yaml
hc-hello-world-plugin:
  id: "hc-hello-world-plugin"
  language: "go"
  title: "Hello World HashiCorp Plugin"
  enable: true
  debug: true # Enable debug mode
  binary_path: "hc-hello-world-plugin"
```

### 2. Debug Script

The `runDebug.sh` script must exist in the plugin directory. This script:

- Sets up the correct environment variables for HashiCorp handshake
- Starts the plugin binary with delve debugger
- Listens on port 40000 for remote debugger connections

### 3. VSCode Debug Configuration

Use one of these debug configurations in VSCode:

#### Option A: Start Engine + Auto-attach to Plugin

1. Use compound configuration: `Debug Engine + Attach to Plugin`
2. This will start the engine and automatically try to attach to the plugin debugger

#### Option B: Manual two-step debugging

1. Start the engine: Use `Engine Debug` configuration
2. Attach to plugin: Use `Attach to HashiCorp Plugin (Remote Debug)` configuration

## Debug Process

### Step 1: Start Engine in Debug Mode

1. Open VSCode
2. Go to Run and Debug panel (Ctrl+Shift+D)
3. Select `Engine Debug` from the dropdown
4. Click the green play button
5. The engine will start and load plugins
6. If a plugin has `debug: true`, it will use `runDebug.sh` and wait for debugger

### Step 2: Attach to Plugin Debugger

1. Wait for the engine to start (you'll see delve waiting message)
2. In VSCode, select `Attach to HashiCorp Plugin (Remote Debug)`
3. Click the green play button
4. VSCode will connect to the plugin's delve debugger on port 40000

### Step 3: Debug the Plugin

1. Set breakpoints in your plugin code
2. The plugin will pause at breakpoints when executed
3. Use normal VSCode debugging features (step, inspect variables, etc.)

## Important Notes

### Plugin must be built with debug symbols

Ensure your plugin is built with debug symbols:

```bash
go build -gcflags="all=-N -l" -o hc-hello-world-plugin main.go
```

### Port Configuration

- Engine runs on its configured port
- Plugin debugger runs on port 40000
- Make sure port 40000 is available

### Troubleshooting

1. **Plugin not starting in debug mode**

   - Check if `runDebug.sh` exists and is executable
   - Check if `debug: true` is set in plugins.yml
   - Check logs for error messages

2. **Cannot attach to debugger**

   - Ensure port 40000 is not in use
   - Wait for the "delve is starting" message before attaching
   - Check if delve is properly installed

3. **Breakpoints not working**
   - Ensure plugin is built with debug symbols
   - Check if the source path mapping is correct
   - Try restarting both engine and debugger

## Files Involved

- `plugins.yml` - Plugin configuration with debug flag
- `runDebug.sh` - Debug launcher script (must be executable)
- `.vscode/launch.json` - VSCode debug configurations
- Plugin source code with your breakpoints

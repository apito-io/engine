# 🐛 HashiCorp Plugin Debug Guide

A complete guide for debugging HashiCorp plugins in the Apito Engine using delve and VSCode.

## 📋 Prerequisites

- **delve** debugger installed: `go install github.com/go-delve/delve/cmd/dlv@latest`
- **VSCode** with Go extension
- **Engine** configured with `debug: true` in `plugins.yml`

## 🚀 Quick Start

1. **Start the Engine**:

   ```bash
   cd /path/to/engine
   go run .
   ```

2. **Look for Plugin PID** (shown in colored output when debug enabled):

   ```
   🐛 [DEBUG] Plugin PID: 12345 - Ready for delve attachment!
   ```

3. **Attach Delve**:

   ```bash
   cd plugins/hc-hello-world-plugin
   ./runDebug.sh 12345
   # or auto-detect PID:
   ./runDebug.sh
   ```

4. **Connect VSCode**:

   - Open Debug panel (⇧⌘D)
   - Select "Attach to HashiCorp Plugin (Remote Debug)"
   - Click play button ▶️

5. **Start Debugging**:
   - Set breakpoints in resolver functions
   - Send GraphQL queries from your client
   - Debug queries in real-time!

## 🔧 Configuration

### Engine Configuration (`plugins.yml`)

```yaml
hc-hello-world-plugin:
  id: "hc-hello-world-plugin"
  language: "go"
  title: "Hello World HashiCorp Plugin"
  enable: true
  debug: true # 🔥 Enable this for debugging
  binary_path: "hc-hello-world-plugin"
```

### VSCode Configuration (`.vscode/launch.json`)

```json
{
  "name": "Attach to HashiCorp Plugin (Remote Debug)",
  "type": "go",
  "request": "attach",
  "mode": "remote",
  "remotePath": "",
  "port": 40000,
  "host": "127.0.0.1",
  "showLog": true,
  "logOutput": "debugger",
  "trace": "verbose"
}
```

## 🎯 Debug Workflow

### Step 1: Start Engine

```bash
go run .
```

**Expected Output:**

```
🐛 [DEBUG] Plugin PID: 12345 - Ready for delve attachment!
🔌 [DEBUG] Use: ./runDebug.sh 12345
📝 [DEBUG] Then connect VSCode debugger to localhost:40000
```

### Step 2: Attach Delve

```bash
./runDebug.sh 12345
```

**Expected Output:**

```
🔍 Checking if port 40000 is already in use...
✅ Port 40000 is available
🐛 Attaching delve to plugin process PID: 12345
🔌 Delve will listen on port 40000
📝 Use 'Attach to HashiCorp Plugin (Remote Debug)' in VSCode
❌ Press Ctrl+C to stop debugging and cleanup
```

### Step 3: Connect VSCode

1. Open VSCode Debug panel
2. Select "Attach to HashiCorp Plugin (Remote Debug)"
3. Click play button
4. VSCode connects to delve on port 40000

### Step 4: Set Breakpoints

```go
func helloWorldResolver(ctx context.Context, rawArgs map[string]interface{}) (interface{}, error) {
    // 🔴 Set breakpoint here
    log.Printf("🚀 [DEBUG] helloWorldResolver called")

    // Your resolver logic...
    return result, nil
}
```

### Step 5: Send Test Query

```graphql
query {
  helloWorldQueryFahim(name: "Debug Test") {
    message
  }
}
```

## 🛠️ Debug Scripts

### `runDebug.sh` - Main Debug Script

**Features:**

- ✅ Auto port conflict resolution
- ✅ PID auto-detection
- ✅ Graceful cleanup on Ctrl+C
- ✅ Clear status messages

**Usage:**

```bash
./runDebug.sh [PID]    # Specific PID
./runDebug.sh          # Auto-detect PID
```

### `killDebug.sh` - Cleanup Utility

**Features:**

- 🔪 Kills delve processes on port 40000
- 🧹 Cleans up hanging dlv processes
- ✅ Safe and thorough cleanup

**Usage:**

```bash
./killDebug.sh
```

## 🔍 Debugging Tips

### Setting Effective Breakpoints

```go
// 1. Resolver entry points
func helloWorldResolver(ctx context.Context, rawArgs map[string]interface{}) (interface{}, error) {
    // 🔴 Breakpoint: Check incoming arguments

// 2. Business logic sections
    result := processBusinessLogic(args)
    // 🔴 Breakpoint: Inspect processed data

// 3. Error conditions
    if err != nil {
        // 🔴 Breakpoint: Debug error handling
        return nil, err
    }
}
```

### Inspecting Context Data

```go
// Debug context values passed from host
pluginID := sdk.GetPluginID(rawArgs)     // Plugin identifier
projectID := sdk.GetProjectID(rawArgs)   // Project context
userID := sdk.GetUserID(rawArgs)         // User context
tenantID := sdk.GetTenantID(rawArgs)     // Tenant context
```

### Testing Different Query Types

```graphql
# Simple query
query {
  helloWorldQueryFahim(name: "test")
}

# Complex object query
query {
  getUserProfile(userId: "123") {
    id
    name
    email
  }
}

# Mutation with input
mutation {
  createUser(input: { name: "John", email: "john@example.com" })
}
```

## 🚨 Troubleshooting

### Port Already in Use

**Problem:** `bind: address already in use`
**Solution:**

```bash
./killDebug.sh          # Clean up existing sessions
./runDebug.sh [PID]     # Try again
```

### Plugin Process Not Found

**Problem:** `Plugin process not found`
**Solutions:**

1. Check engine is running: `ps aux | grep "go run"`
2. Verify plugin loaded: Check engine logs
3. Use specific PID: `./runDebug.sh [PID]`

### VSCode Can't Connect

**Problem:** Connection timeout or refused
**Solutions:**

1. Verify delve is listening: `lsof -i :40000`
2. Check firewall settings
3. Restart debug session: Ctrl+C → `./runDebug.sh`

### Breakpoints Not Hit

**Possible Causes:**

1. Plugin not compiled with debug symbols: `go build -gcflags="all=-N -l"`
2. Breakpoint in unreachable code
3. Query not triggering the resolver

## 📚 Advanced Debugging

### Debug with Request Context

```go
// Extract full context for debugging
allContext := sdk.GetAllContextData(rawArgs)
log.Printf("🔍 Full context: %+v", allContext)
```

### Debug Complex Arguments

```go
// Parse and inspect arguments
args := sdk.ParseArgsForResolver("helloWorldQuery", rawArgs)
log.Printf("📝 Parsed args: %+v", args)

// Check specific argument types
if nameArg, ok := args["name"].(string); ok {
    log.Printf("📄 Name argument: %s", nameArg)
}
```

### Performance Debugging

```go
start := time.Now()
defer func() {
    duration := time.Since(start)
    log.Printf("⏱️ Resolver execution time: %v", duration)
}()
```

## 🎉 Success Indicators

- ✅ Engine starts without errors
- ✅ Plugin PID displayed in colored format
- ✅ Delve attaches successfully
- ✅ VSCode connects to debugger
- ✅ Breakpoints hit when queries sent
- ✅ Can inspect variables and context
- ✅ Can step through code execution

## 🤝 Getting Help

If you encounter issues:

1. Check this guide for common solutions
2. Verify all prerequisites are installed
3. Ensure engine configuration is correct
4. Try the cleanup scripts: `./killDebug.sh`
5. Restart the entire debug session

Happy debugging! 🐛✨

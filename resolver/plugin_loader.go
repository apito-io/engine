package resolver

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/apito-io/engine/models"
	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types/protobuff"
	"github.com/hashicorp/go-hclog"
	hcplugin "github.com/hashicorp/go-plugin"
	"github.com/labstack/echo/v4"
	"github.com/tailor-inc/graphql"
	"google.golang.org/protobuf/types/known/structpb"
)

const PROJECT_PLUGIN_PREFIX = "plg"
const DEFAULT_INTERNAL_PREFIX = "ext"

// ========================================
// ERROR HANDLING WITH HTTP STATUS CODES
// ========================================

// CodedError represents an error with an HTTP status code (mirrors SDK structure)
type CodedError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

func (e *CodedError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("HTTP %d: %s (%s)", e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("HTTP %d: %s", e.Code, e.Message)
}

// extractErrorCode attempts to extract HTTP status code from error message
func extractErrorCode(err error) int {
	if err == nil {
		return 200
	}

	errMsg := err.Error()

	// Try to parse "HTTP XXX:" pattern from SDK CodedError
	if strings.HasPrefix(errMsg, "HTTP ") {
		parts := strings.SplitN(errMsg, ":", 2)
		if len(parts) >= 1 {
			codeStr := strings.TrimPrefix(parts[0], "HTTP ")
			codeStr = strings.TrimSpace(codeStr)
			if code, parseErr := strconv.Atoi(codeStr); parseErr == nil {
				return code
			}
		}
	}

	// Default to 500 for unknown errors
	return 500
}

// setResponseStatus sets the HTTP response status code based on error
func setResponseStatus(c echo.Context, err error) {
	if err != nil {
		statusCode := extractErrorCode(err)
		c.Response().WriteHeader(statusCode)
	}
}

// convertProtobufValueToGo converts a protobuf JSON value to a Go object
func convertProtobufValueToGo(value interface{}) interface{} {
	if valueMap, ok := value.(map[string]interface{}); ok {
		// Handle structValue
		if structValue, exists := valueMap["structValue"]; exists {
			if structMap, ok := structValue.(map[string]interface{}); ok {
				if fields, exists := structMap["fields"]; exists {
					if fieldsMap, ok := fields.(map[string]interface{}); ok {
						result := make(map[string]interface{})
						for fieldName, fieldValue := range fieldsMap {
							result[fieldName] = convertProtobufValueToGo(fieldValue)
						}
						return result
					}
				}
			}
		}

		// Handle listValue
		if listValue, exists := valueMap["listValue"]; exists {
			if listMap, ok := listValue.(map[string]interface{}); ok {
				if values, exists := listMap["values"]; exists {
					if valuesList, ok := values.([]interface{}); ok {
						result := make([]interface{}, len(valuesList))
						for i, item := range valuesList {
							result[i] = convertProtobufValueToGo(item)
						}
						return result
					}
				}
			}
		}

		// Handle primitive values
		if stringValue, exists := valueMap["stringValue"]; exists {
			return stringValue
		}
		if numberValue, exists := valueMap["numberValue"]; exists {
			return numberValue
		}
		if boolValue, exists := valueMap["boolValue"]; exists {
			// Ensure boolean values are returned as actual booleans, not strings
			if boolVal, ok := boolValue.(bool); ok {
				return boolVal
			}
			// Handle string representations of booleans
			if strVal, ok := boolValue.(string); ok {
				return strVal == "true"
			}
			return boolValue
		}
		if _, exists := valueMap["nullValue"]; exists {
			return nil
		}
	}

	// Return as-is if not a recognized protobuf structure
	return value
}

func (s *GraphQLServer) LoadPlugins(ctx context.Context) error {
	// Load HashiCorp plugins only
	err := s.LoadHashiCorpPlugins(ctx)
	if err != nil {
		fmt.Printf("Error loading HashiCorp plugins: %v\n", err)
		// Don't return error to allow the system to continue with existing plugins
	}

	return nil
}

// HashiCorp Plugin Loading Functions

func (s *GraphQLServer) LoadHashiCorpPlugins(ctx context.Context) error {

	// Load the plugin registry once at the beginning
	_hashiCorpPlugins, err := pluginService.LoadHashiCorpPluginRegistry(s.Cfg)
	if err != nil {
		return fmt.Errorf("failed to load HashiCorp plugin registry: %w", err)
	}

	// Ensure plugin directory exists
	if err = os.MkdirAll(s.Cfg.PluginPath, 0770); err != nil {
		return fmt.Errorf("failed to create plugin directory: %w", err)
	}

	var loadedPlugins []*protobuff.PluginDetails

	// Load plugins based on registry entries instead of directory scanning
	for pluginName, pluginDetails := range _hashiCorpPlugins {

		s.wg.Add(1)

		go func(_pluginName string, _pluginDetails *protobuff.PluginDetails) {
			errs := func() []error {
				defer s.wg.Done()
				var errs []error

				// Check if plugin is already loaded
				if _, ok := s.HashiCorpPluginCache[_pluginName]; ok {
					fmt.Printf("HashiCorp Plugin %s already loaded, skipping\n", _pluginName)
					return errs
				}

				// Check if plugin is enabled before loading
				if !_pluginDetails.Enable {
					fmt.Printf("HashiCorp Plugin %s is disabled, skipping\n", _pluginName)
					return errs
				}

				// Construct plugin directory path
				dir := filepath.Join(s.Cfg.PluginPath, _pluginName)

				// Check if plugin directory exists
				if _, err := os.Stat(dir); os.IsNotExist(err) {
					errs = append(errs, fmt.Errorf("HashiCorp Plugin %s directory not found: %s", _pluginName, dir))
					return errs
				}

				fmt.Printf("Loading HashiCorp Plugin %s\n", _pluginName)

				/* // for the first time plugin loader. there is no project driver
				envs, err := s.GetInjectableSystemVariable(driverCred)
				if err != nil {
					errs = append(errs, err)
				}

				_pluginDetails.EnvVars = append(_pluginDetails.EnvVars, envs...) */

				loadedPlugin, err := s.LoadHashiCorpPlugin(context.Background(), dir, _pluginDetails)
				if err != nil {
					errs = append(errs, err)
				} else {
					loadedPlugins = append(loadedPlugins, loadedPlugin)
				}

				return errs
			}()

			if len(errs) > 0 {
				for _, err := range errs {
					fmt.Printf("Error Loading HashiCorp Plugin %s: %s\n", _pluginName, err.Error())
				}
			}
		}(pluginName, pluginDetails)
	}

	return nil
}

func (s *GraphQLServer) CheckHashiCorpPluginExists(dir string) (int32, error) {
	// Load plugin registry to get configuration
	_hashiCorpPlugins, err := pluginService.LoadHashiCorpPluginRegistry(s.Cfg)
	if err != nil {
		return 2, fmt.Errorf("failed to load plugin registry: %w", err)
	}

	return s.CheckHashiCorpPluginExistsWithRegistry(dir, _hashiCorpPlugins)
}

func (s *GraphQLServer) CheckHashiCorpPluginExistsWithRegistry(dir string, _hashiCorpPlugins map[string]*protobuff.PluginDetails) (int32, error) {
	// Get plugin ID from directory name
	pluginID := filepath.Base(dir)

	// Check if plugin is defined in registry
	pluginDetails, exists := _hashiCorpPlugins[pluginID]
	if !exists {
		return 0, fmt.Errorf("plugin %s not found in registry", pluginID)
	}

	// Check if the plugin file exists based on its binary_path
	binaryPath := filepath.Join(dir, pluginDetails.BinaryPath)

	if _, err := os.Stat(binaryPath); err == nil {
		return 1, nil
	} else {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("plugin binary not found: %s", binaryPath)
		}
		return 2, err
	}
}

// LaunchPluginByLanguage creates and returns the appropriate command based on plugin language
// TODO: This function needs a better way to handle:
//   - Plugin launch process isolation
//   - Resource management and cleanup
//   - Health monitoring and auto-restart
//   - Security sandboxing
//   - Docker-based plugin execution for better isolation
//   - Plugin lifecycle management (start, stop, restart, health checks)
//   - Multi-platform support (Windows, Linux, macOS)
//   - Plugin dependency management
//   - Crash recovery and automatic restart
//   - Resource limits (CPU, memory, file descriptors)
//   - Network isolation and security policies
//     For now, this provides basic language-aware launching
func (s *GraphQLServer) LaunchPluginByLanguage(pluginDetails *protobuff.PluginDetails, pluginDir string) (*exec.Cmd, error) {
	binaryPath := filepath.Join(pluginDir, pluginDetails.BinaryPath)

	// Convert to absolute path to avoid working directory issues
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for plugin %s: %w", binaryPath, err)
	}

	// Check if plugin exists using absolute path
	if _, err := os.Stat(absBinaryPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("plugin not found [%s]. please check path", absBinaryPath)
	}

	var cmd *exec.Cmd

	switch pluginDetails.Language {
	case protobuff.PluginLanguage_PLUGIN_LANGUAGE_GO:
		// Go binary - check for debug mode and runDebug.sh script
		debugMode := strconv.FormatBool(pluginDetails.Debug)
		debugScriptPath := filepath.Join(pluginDir, "runDebug.sh")

		if debugMode == "true" {
			// Check if runDebug.sh exists as a marker file
			if _, err := os.Stat(debugScriptPath); err == nil {
				// Start plugin normally to ensure HashiCorp handshake works
				// Delve attachment will be handled separately
				cmd = exec.Command(absBinaryPath)
			} else {
				return nil, fmt.Errorf("DEBUG MODE ENABLED but runDebug.sh not found in plugin directory: %s. Please create the debug script", debugScriptPath)
			}
		} else {
			// Normal Go binary execution with full path
			cmd = exec.Command(absBinaryPath)
		}

	case protobuff.PluginLanguage_PLUGIN_LANGUAGE_JS:
		// JavaScript - run with Node.js using relative path since we set working directory
		cmd = exec.Command("node", absBinaryPath)

	case protobuff.PluginLanguage_PLUGIN_LANGUAGE_PYTHON:
		// Python - run with Python interpreter using relative path
		cmd = exec.Command("python3", pluginDetails.BinaryPath)

	case protobuff.PluginLanguage_PLUGIN_LANGUAGE_CPP:
		// C++ binary - direct execution (assuming compiled) with full path
		cmd = exec.Command(absBinaryPath)

	case protobuff.PluginLanguage_PLUGIN_LANGUAGE_JAVA:
		// Java - run with Java runtime using relative path
		cmd = exec.Command("java", "-jar", pluginDetails.BinaryPath)

	case protobuff.PluginLanguage_PLUGIN_LANGUAGE_CSHARP:
		// C# - run with dotnet runtime using relative path
		cmd = exec.Command("dotnet", pluginDetails.BinaryPath)

	case protobuff.PluginLanguage_PLUGIN_LANGUAGE_PHP:
		// PHP - run with PHP interpreter using relative path
		cmd = exec.Command("php", pluginDetails.BinaryPath)

	case protobuff.PluginLanguage_PLUGIN_LANGUAGE_RUBY:
		// Ruby - run with Ruby interpreter using relative path
		cmd = exec.Command("ruby", pluginDetails.BinaryPath)

	case protobuff.PluginLanguage_PLUGIN_LANGUAGE_TYPESCRIPT:
		// TypeScript - run with Node.js using relative path
		cmd = exec.Command("node", pluginDetails.BinaryPath)

	default:
		return nil, fmt.Errorf("unsupported plugin language: %v", pluginDetails.Language)
	}

	// Set environment variables for the plugin
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}

	// Add plugin-specific environment variables
	for _, envVar := range pluginDetails.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", envVar.Key, envVar.Value))
	}

	// Add debug environment variable when debug mode is enabled
	if pluginDetails.Debug {
		cmd.Env = append(cmd.Env, "PLUGIN_DEBUG_MODE=true")
	}

	// Set working directory to plugin directory
	cmd.Dir = pluginDir

	return cmd, nil
}

func (s *GraphQLServer) LoadHashiCorpPlugin(ctx context.Context, _dir string, _pluginDetails *protobuff.PluginDetails) (*protobuff.PluginDetails, error) {

	// Use the new language-aware launcher
	cmd, err := s.LaunchPluginByLanguage(_pluginDetails, _dir)
	if err != nil {
		return nil, fmt.Errorf("failed to create plugin command: %w", err)
	}

	// Create logger for the plugin
	logger := hclog.New(&hclog.LoggerOptions{
		Name:   _pluginDetails.Id,
		Output: os.Stdout,
		Level:  hclog.Debug,
	})

	// Create plugin map - all plugins now use the universal plugin service
	pluginMap := make(map[string]hcplugin.Plugin)

	// Use the same universal plugin RPC for all plugin types
	pluginMap[_pluginDetails.ExportedVariable] = &pluginService.HashiCorpNormalPluginGRPC{}

	// Determine AutoMTLS setting based on plugin language
	// JavaScript and other non-Go plugins may not support AutoMTLS properly
	autoMTLS := true
	if _pluginDetails.Language == protobuff.PluginLanguage_PLUGIN_LANGUAGE_JS ||
		_pluginDetails.Language == protobuff.PluginLanguage_PLUGIN_LANGUAGE_PYTHON ||
		_pluginDetails.Language == protobuff.PluginLanguage_PLUGIN_LANGUAGE_TYPESCRIPT {
		autoMTLS = false
		fmt.Printf("🔓 [PLUGIN-LOADER] Disabling AutoMTLS for %s plugin: %s\n", _pluginDetails.Language.String(), _pluginDetails.Id)
	}

	// Create client with explicit GRPC protocol support
	client := hcplugin.NewClient(&hcplugin.ClientConfig{
		HandshakeConfig: hcplugin.HandshakeConfig{
			ProtocolVersion:  uint(_pluginDetails.HandshakeConfig.ProtocolVersion),
			MagicCookieKey:   _pluginDetails.HandshakeConfig.MagicCookieKey,
			MagicCookieValue: _pluginDetails.HandshakeConfig.MagicCookieValue,
		},
		Plugins: pluginMap,
		Cmd:     cmd,
		Logger:  logger,
		// Explicitly allow both protocols, with gRPC being preferred
		AllowedProtocols: []hcplugin.Protocol{
			hcplugin.ProtocolNetRPC,
			hcplugin.ProtocolGRPC,
		},
		AutoMTLS: autoMTLS,
	})

	// If Go plugin is in debug mode, store PID for later delve attachment
	if _pluginDetails.Debug && _pluginDetails.Language == protobuff.PluginLanguage_PLUGIN_LANGUAGE_GO {
		// Give plugin time to start before attempting attachment
		time.Sleep(2 * time.Second)
	}

	// Connect via RPC
	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to create RPC client for plugin %s: %w", _pluginDetails.Id, err)
	}

	// Get plugin instance
	raw, err := rpcClient.Dispense(_pluginDetails.ExportedVariable)
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("failed to dispense plugin %s: %w", _pluginDetails.Id, err)
	}

	pluginID := _pluginDetails.Id

	// inject db env vars
	var envs []*protobuff.EnvVariable
	for _, v := range _pluginDetails.EnvVars {
		envs = append(envs, &protobuff.EnvVariable{Key: v.Key, Value: v.Value})
	}

	// Initialize plugin using universal interface
	loadedPlugin, ok := raw.(*pluginService.HashiCorpNormalPluginGRPC)
	if !ok {
		client.Kill()
		return nil, fmt.Errorf("%s HashiCorp plugin load failed - invalid plugin type", pluginID)
	}

	// Convert env vars to protobuf format
	var protoEnvVars []*protobuff.EnvVariable
	for _, env := range envs {
		protoEnvVars = append(protoEnvVars, &protobuff.EnvVariable{
			Key:   env.Key,
			Value: env.Value,
		})
	}

	err = loadedPlugin.Init(ctx, protoEnvVars)
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("%s %s Init Call failed", pluginID, err.Error())
	}

	// Register the plugin schema and APIs with the system
	err = s.registerPluginSchemaAndAPIs(ctx, loadedPlugin, _pluginDetails)
	if err != nil {
		client.Kill()
		return nil, fmt.Errorf("%s schema/API registration failed: %w", pluginID, err)
	}

	if !utility.ArrayContains(s.InstalledHCPluginList, pluginID) {
		s.InstalledHCPluginList = append(s.InstalledHCPluginList, pluginID)
	}

	s.Lock()
	_pluginDetails.LoadStatus = protobuff.PluginLoadStatus_PLUGIN_LOAD_STATUS_LOADED
	s.HashiCorpPluginCache[pluginID] = &models.HashiCorpPluginCache{
		Client:               client,
		PluginConfigurations: _pluginDetails,
		RPCClient:            rpcClient,
	}
	s.Unlock()

	return _pluginDetails, nil
}

// registerPluginSchemaAndAPIs handles the registration of plugin GraphQL schemas and REST APIs
func (s *GraphQLServer) registerPluginSchemaAndAPIs(ctx context.Context, plugin *pluginService.HashiCorpNormalPluginGRPC, pluginDetails *protobuff.PluginDetails) error {
	// 1. Register GraphQL Schemas
	pluginSchemas, err := plugin.SchemaRegister(ctx)
	if err != nil {
		return fmt.Errorf("failed to register plugin schemas: %w", err)
	}

	if pluginSchemas != nil {
		// Convert protobuf schemas to GraphQL fields using server-aware method
		convertedSchemas, err := s.convertProtoGraphQLSchemaToFieldsForSystemSchema(pluginSchemas, pluginDetails.Id)
		if err != nil {
			return fmt.Errorf("failed to convert plugin schemas: %w", err)
		}

		// Register queries
		if convertedSchemas.Queries != nil {
			for k, v := range convertedSchemas.Queries {
				queryKey := DEFAULT_INTERNAL_PREFIX + "_" + k
				if val := s.SystemQueries[queryKey]; val == nil && pluginDetails.Type == protobuff.PluginType_PLUGIN_TYPE_SYSTEM {
					fmt.Printf("--> Registering plugin schema from %s query `%s` to system schema\n", pluginDetails.Id, k)
					s.SystemQueries[queryKey] = v
				} else {
					fmt.Printf("the query '%s' on '%s' already found on another plugin. please change the id. ignoring this query\n", k, pluginDetails.Id)
				}
			}
		}

		// Register mutations
		if convertedSchemas.Mutations != nil {
			for k, v := range convertedSchemas.Mutations {
				mutationKey := DEFAULT_INTERNAL_PREFIX + "_" + k
				if val := s.SystemMutations[mutationKey]; val == nil && pluginDetails.Type == protobuff.PluginType_PLUGIN_TYPE_SYSTEM {
					fmt.Printf("--> Registering plugin schema from %s mutation `%s` to system schema\n", pluginDetails.Id, k)
					s.SystemMutations[mutationKey] = v
				} else {
					fmt.Printf("the mutation '%s' on '%s' already found on another plugin. please change the id. ignoring this mutation\n", k, pluginDetails.Id)
				}
			}
		}
	}

	// 2. Register REST APIs
	pluginAPIs, err := plugin.RESTApiRegister(ctx)
	if err != nil {
		return fmt.Errorf("failed to register plugin APIs: %w", err)
	}

	if len(pluginAPIs) > 0 {
		// Convert protobuf APIs to REST routes
		convertedAPIs := pluginService.ConvertProtoRestApisToLocal(pluginAPIs)

		// Register routes with the extension router
		for _, route := range convertedAPIs {
			var path string
			if pluginDetails.Type == protobuff.PluginType_PLUGIN_TYPE_PROJECT {
				path = "/" + pluginDetails.Id + route.Path
			} else {
				path = "/" + route.Path
			}
			if !utility.ArrayContains(s.ExtensionRouterList, path) {
				fmt.Printf("--> Registering plugin REST API %s %s to system routes\n", route.Method, path)

				// Create a handler that will call the plugin
				handler := s.createPluginAPIHandler(pluginDetails.Id, route)

				// Register the route with the echo router
				s.ExtensionRouter.Add(route.Method, path, handler)
				s.ExtensionRouterList = append(s.ExtensionRouterList, path)

				fmt.Printf("--> Plugin REST API registered %s %s to system routes\n", route.Method, path)
			} else {
				fmt.Printf("--> Plugin REST API already registered %s %s to system routes\n", route.Method, path)
			}
		}

		// Show all registered Echo routes including plugin routes
		fmt.Printf("\n🔌 [PLUGIN-LOADER] Plugin REST APIs registered, showing all Echo routes:\n")
		s.PrintAllPluginRoutes()
	}

	return nil
}

// createPluginAPIHandler creates an Echo handler that routes requests to the plugin
func (s *GraphQLServer) createPluginAPIHandler(pluginID string, route *protobuff.ThirdPartyRESTApi) echo.HandlerFunc {
	return func(c echo.Context) error {
		// Execute the plugin REST handler using the same pattern as GraphQL resolvers
		result, err := s.executePluginRESTHandler(c.Request().Context(), pluginID, route, c)

		// Handle CodedError types from plugins
		if err != nil {
			if codedErr, ok := err.(*CodedError); ok {
				// Plugin returned a coded error - set proper status code
				return c.JSON(codedErr.Code, map[string]interface{}{
					"error":   codedErr.Message,
					"code":    codedErr.Code,
					"details": codedErr.Details,
				})
			}

			// Regular error - internal server error
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": fmt.Sprintf("Plugin %s REST API error: %v", pluginID, err),
				"code":  500,
			})
		}

		// Handle different result types
		switch v := result.(type) {
		case map[string]interface{}:
			// Check if there's a specific status code
			if statusCode, exists := v["_status_code"]; exists {
				if code, ok := statusCode.(float64); ok {
					delete(v, "_status_code") // Remove internal field
					return c.JSON(int(code), v)
				}
			}
			return c.JSON(http.StatusOK, v)
		case string:
			// Handle error messages or simple string responses
			if strings.HasPrefix(v, "❌ ERROR:") || strings.HasPrefix(v, "❌ PLUGIN ERROR:") {
				return c.JSON(http.StatusInternalServerError, map[string]interface{}{
					"error": v,
					"code":  500,
				})
			}
			return c.JSON(http.StatusOK, map[string]interface{}{
				"message": v,
			})
		default:
			return c.JSON(http.StatusOK, map[string]interface{}{
				"data": result,
			})
		}
	}
}

// executePluginRESTHandler executes a REST API call on a HashiCorp plugin
// This follows the same pattern as executePluginGraphQLResolver but for REST APIs
func (s *GraphQLServer) executePluginRESTHandler(ctx context.Context, pluginID string, route *protobuff.ThirdPartyRESTApi, echoCtx echo.Context) (interface{}, error) {
	// Try to get plugin from server cache directly (with proper locking)
	plugin := s.tryGetPluginNoBlock(pluginID)
	if plugin == nil {
		return fmt.Sprintf("❌ ERROR: Plugin %s not available", pluginID), nil
	}

	// Get the RPC client
	rpcClient, err := plugin.Client.Client()
	if err != nil {
		return fmt.Sprintf("❌ ERROR: Failed to get RPC client for %s: %v", pluginID, err), nil
	}

	// Dispense the plugin
	raw, err := rpcClient.Dispense(plugin.PluginConfigurations.ExportedVariable)
	if err != nil {
		return fmt.Sprintf("❌ ERROR: Failed to dispense plugin %s: %v", pluginID, err), nil
	}

	loadedPlugin, ok := raw.(*pluginService.HashiCorpNormalPluginGRPC)
	if !ok {
		return fmt.Sprintf("❌ ERROR: Plugin %s is not a valid universal plugin", pluginID), nil
	}

	// Prepare REST API arguments from Echo context
	args := make(map[string]interface{})

	// Add query parameters
	for key, values := range echoCtx.QueryParams() {
		if len(values) == 1 {
			args[key] = values[0]
		} else {
			args[key] = values
		}
	}

	// Add path parameters
	paramNames := echoCtx.ParamNames()
	for _, name := range paramNames {
		args[name] = echoCtx.Param(name)
	}

	// Add request body for POST/PUT/PATCH requests
	if route.Method == "POST" || route.Method == "PUT" || route.Method == "PATCH" {
		// Check if this is a multipart form (for file uploads)
		contentType := echoCtx.Request().Header.Get("Content-Type")
		if strings.HasPrefix(contentType, "multipart/form-data") {
			// Handle multipart form data (including file uploads)
			if err := echoCtx.Request().ParseMultipartForm(32 << 20); err == nil { // 32 MB max memory
				// Add form fields as body parameters (convert []string to string for single values)
				for key, values := range echoCtx.Request().MultipartForm.Value {
					if len(values) == 1 {
						args["body_"+key] = values[0] // Single value as string
					} else {
						args["body_"+key] = values // Multiple values as []string
					}
				}

				// Handle file uploads
				if len(echoCtx.Request().MultipartForm.File) > 0 {
					fileUploads := make(map[string]interface{})
					for fieldName, files := range echoCtx.Request().MultipartForm.File {
						if len(files) > 0 {
							fileHeader := files[0] // Take the first file for each field
							file, err := fileHeader.Open()
							if err == nil {
								// Read file content
								fileContent := make([]byte, fileHeader.Size)
								if _, err := file.Read(fileContent); err == nil {
									fileUploads[fieldName] = map[string]interface{}{
										"filename":     fileHeader.Filename,
										"size":         fileHeader.Size,
										"content_type": fileHeader.Header.Get("Content-Type"),
										"content":      fileContent,
									}
								}
								file.Close()
							}
						}
					}
					if len(fileUploads) > 0 {
						args["file_uploads"] = fileUploads
					}
				}
			}
		} else {
			// Handle regular JSON body
			var requestBody map[string]interface{}
			if err := echoCtx.Bind(&requestBody); err == nil && requestBody != nil {
				// Merge body data into args, prefixing with "body_" to avoid conflicts
				for key, value := range requestBody {
					args["body_"+key] = value
				}
				// Also add the entire body as a single field
				args["_request_body"] = requestBody
			}
		}
	}

	// Add HTTP method and path to args
	args["_http_method"] = route.Method
	args["_http_path"] = route.Path

	// Call the actual Execute method

	// Prepare context data to pass sensitive values
	contextData := map[string]interface{}{
		"plugin_id":   pluginID,
		"http_method": route.Method,
		"http_path":   route.Path,
	}

	// Extract additional context values if needed
	if projectID := ctx.Value("project_id"); projectID != nil {
		contextData["project_id"] = projectID
	}
	if userID := ctx.Value("user_id"); userID != nil {
		contextData["user_id"] = userID
	}
	if tenantID := ctx.Value("tenant_id"); tenantID != nil {
		contextData["tenant_id"] = tenantID
	}

	// Create a unique function name based on the route
	functionName := fmt.Sprintf("rest_%s_%s", strings.ToLower(route.Method), strings.ReplaceAll(strings.TrimPrefix(route.Path, "/"), "/", "_"))
	if functionName == "rest__" {
		functionName = "rest_root"
	}

	response, err := loadedPlugin.Execute(ctx, functionName, "rest_api", args, contextData)
	if err != nil {
		return fmt.Sprintf("❌ ERROR: Execute failed: %v", err), nil
	}

	if !response.Success {
		return fmt.Sprintf("❌ PLUGIN ERROR: %s", response.Message), nil
	}

	// Parse the result from protobuf Any
	if response.Result != nil {
		// First try to unmarshal as structpb.Struct (existing approach)
		var resultStruct structpb.Struct
		if err := response.Result.UnmarshalTo(&resultStruct); err == nil {
			resultMap := resultStruct.AsMap()

			// Check if this is JSON-serialized complex data
			if serialization, exists := resultMap["serialization"]; exists && serialization == "json_bytes" {
				log.Printf("🎯 [ENGINE] Detected JSON-serialized complex data from plugin %s", pluginID)
				if data, exists := resultMap["data"]; exists {
					return data, nil
				}
			} else {
				// Regular struct data - check for error conditions first
				if data, exists := resultMap["data"]; exists {
					// Check if the data contains error information
					if dataMap, ok := data.(map[string]interface{}); ok {
						// Check for success field indicating error
						if success, exists := dataMap["success"]; exists {
							if successBool, ok := success.(bool); ok && !successBool {
								// This is an error response, extract error details
								errorMsg := "Unknown error"
								statusCode := 500

								if errMsg, exists := dataMap["error"]; exists {
									if errStr, ok := errMsg.(string); ok {
										errorMsg = errStr
									}
								}

								if code, exists := dataMap["code"]; exists {
									if codeFloat, ok := code.(float64); ok {
										statusCode = int(codeFloat)
									} else if codeInt, ok := code.(int); ok {
										statusCode = codeInt
									}
								}

								// Return a coded error instead of treating as success
								log.Printf("🚨 [ENGINE] Plugin %s returned error: %s (code: %d)", pluginID, errorMsg, statusCode)
								return nil, &CodedError{
									Code:    statusCode,
									Message: errorMsg,
								}
							}
						}
					}

					// No error detected, return data normally
					return data, nil
				}
			}
		}

		// Try to unmarshal as structpb.Value (handles arrays, primitives, etc.)
		var resultValue structpb.Value
		if err := response.Result.UnmarshalTo(&resultValue); err == nil {
			// Handle different value types from structpb.Value
			if stringValue := resultValue.GetStringValue(); stringValue != "" {
				log.Printf("🎯 [ENGINE] Detected JSON string value from plugin %s", pluginID)

				// Try to parse as JSON
				var jsonData map[string]interface{}
				if err := json.Unmarshal([]byte(stringValue), &jsonData); err == nil {
					// Check if this is our JSON-serialized complex data
					if serialization, exists := jsonData["serialization"]; exists && serialization == "json_bytes" {
						log.Printf("✅ [ENGINE] Successfully parsed JSON-serialized complex data from plugin %s", pluginID)
						if data, exists := jsonData["data"]; exists {
							return data, nil
						}
					}
				} else {
					log.Printf("❌ [ENGINE] Failed to parse JSON from plugin %s: %v", pluginID, err)
				}
			} else if listValue := resultValue.GetListValue(); listValue != nil {
				log.Printf("🎯 [ENGINE] Detected list value from plugin %s", pluginID)
				// Convert protobuf ListValue to Go slice
				result := make([]interface{}, len(listValue.Values))
				for i, val := range listValue.Values {
					result[i] = val.AsInterface()
				}
				log.Printf("✅ [ENGINE] Successfully parsed list with %d items from plugin %s", len(result), pluginID)
				return result, nil
			} else if structValue := resultValue.GetStructValue(); structValue != nil {
				log.Printf("🎯 [ENGINE] Detected struct value from plugin %s", pluginID)
				// Convert protobuf Struct to Go map
				result := structValue.AsMap()
				log.Printf("✅ [ENGINE] Successfully parsed struct from plugin %s", pluginID)
				return result, nil
			} else if _, isNumber := resultValue.GetKind().(*structpb.Value_NumberValue); isNumber {
				numberValue := resultValue.GetNumberValue()
				log.Printf("🎯 [ENGINE] Detected number value from plugin %s: %f", pluginID, numberValue)
				return numberValue, nil
			} else if _, isBool := resultValue.GetKind().(*structpb.Value_BoolValue); isBool {
				boolValue := resultValue.GetBoolValue()
				log.Printf("🎯 [ENGINE] Detected boolean value from plugin %s: %t", pluginID, boolValue)
				return boolValue, nil
			} else if _, isNull := resultValue.GetKind().(*structpb.Value_NullValue); isNull {
				log.Printf("🎯 [ENGINE] Detected null value from plugin %s", pluginID)
				return nil, nil
			}
		}

		log.Printf("⚠️ [ENGINE] Could not parse result from plugin %s", pluginID)
	}

	return fmt.Sprintf("✅ SUCCESS: Plugin %s.Execute(%s) completed", pluginID, functionName), nil
}

// LoadProjectSpecificPlugins loads HashiCorp plugins specific to a project into the ApplicationCache
func (s *GraphQLServer) LoadProjectSpecificPlugins(ctx context.Context, cache *models.ApplicationCache) error {
	project := cache.Project
	if project == nil {
		return nil
	}

	// Initialize plugin cache fields if not exist
	if cache.RawSchemas == nil {
		cache.RawSchemas = &models.RawSchema{
			Queries:   make(graphql.Fields),
			Mutations: make(graphql.Fields),
		}
	}

	// Collect enabled HashiCorp plugins
	var enabledPlugins []*protobuff.PluginDetails
	for _, pluginDetails := range project.Plugins {
		if pluginDetails.Enable && strings.HasPrefix(pluginDetails.Id, "hc-") {
			enabledPlugins = append(enabledPlugins, pluginDetails)
		}
	}

	if len(enabledPlugins) == 0 {
		return nil
	}

	// DEADLOCK-FREE APPROACH: Use TryLock with immediate fallback
	for _, pluginDetails := range enabledPlugins {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Try to get plugin without blocking - if we can't get it immediately, skip it
		globalPlugin := s.tryGetPluginNoBlock(pluginDetails.Id)

		if globalPlugin != nil {
			// Register the existing plugin to this project's cache (this should be lock-free)
			err := s.registerProjectPluginToCache(ctx, cache, globalPlugin, pluginDetails)
			if err != nil {
				fmt.Printf("[ERROR] LoadProjectSpecificPlugins: Failed to register plugin %s for project %s: %v\n", pluginDetails.Id, project.ID, err)
			}
		}
	}

	return nil
}

// tryGetPluginNoBlock attempts to get a plugin from global cache without blocking
// Returns nil if plugin doesn't exist or if we can't get lock immediately
func (s *GraphQLServer) tryGetPluginNoBlock(pluginID string) *models.HashiCorpPluginCache {
	// Create a channel to receive the result
	resultChan := make(chan *models.HashiCorpPluginCache, 1)

	// Try to get the plugin in a goroutine with timeout
	go func() {
		defer func() {
			if r := recover(); r != nil {
				resultChan <- nil
			}
		}()

		// Try to acquire lock with a very short timeout
		done := make(chan bool, 1)
		var result *models.HashiCorpPluginCache

		go func() {
			s.Lock()
			if plugin, exists := s.HashiCorpPluginCache[pluginID]; exists {
				result = plugin
			}
			s.Unlock()
			done <- true
		}()

		// Wait for either completion or timeout
		select {
		case <-done:
			resultChan <- result
		case <-time.After(10 * time.Millisecond): // Very short timeout
			resultChan <- nil
		}
	}()

	// Wait for result with timeout
	select {
	case result := <-resultChan:
		return result
	case <-time.After(50 * time.Millisecond): // Overall timeout
		return nil
	}
}

// registerProjectPluginToCache registers a plugin's schemas and APIs to the project cache
func (s *GraphQLServer) registerProjectPluginToCache(ctx context.Context, cache *models.ApplicationCache, globalPlugin *models.HashiCorpPluginCache, pluginDetails *protobuff.PluginDetails) error {
	pluginID := pluginDetails.Id

	// Defensive programming: ensure cache is valid
	if cache == nil || cache.RawSchemas == nil {
		return fmt.Errorf("invalid cache for plugin %s registration", pluginID)
	}

	// Get the RPC client (this operation should not hold locks)
	rpcClient, err := globalPlugin.Client.Client()
	if err != nil {
		return fmt.Errorf("failed to get RPC client for plugin %s: %w", pluginID, err)
	}

	// Dispense the plugin (this operation should not hold locks)
	raw, err := rpcClient.Dispense(globalPlugin.PluginConfigurations.ExportedVariable)
	if err != nil {
		return fmt.Errorf("failed to dispense plugin %s: %w", pluginID, err)
	}

	loadedPlugin, ok := raw.(*pluginService.HashiCorpNormalPluginGRPC)
	if !ok {
		return fmt.Errorf("plugin %s is not a valid universal plugin", pluginID)
	}

	// Register GraphQL schemas (this should not require server locks since it's per-project cache)
	pluginSchemas, err := loadedPlugin.SchemaRegister(ctx)
	if err != nil {
		return fmt.Errorf("failed to register schemas for plugin %s: %w", pluginID, err)
	}

	if pluginSchemas != nil {
		// Convert protobuf schemas to GraphQL fields using server-aware method
		convertedSchemas, err := s.convertProtoGraphQLSchemaToFieldsWithServer(pluginSchemas, pluginID)
		if err != nil {
			return fmt.Errorf("failed to convert schemas for plugin %s: %w", pluginID, err)
		}

		// Add queries to project cache (no locks needed - this is project-specific cache)
		if convertedSchemas.Queries != nil {
			for k, v := range convertedSchemas.Queries {
				queryKey := fmt.Sprintf("%s_%s", PROJECT_PLUGIN_PREFIX, k)
				if cache.RawSchemas.Queries[queryKey] == nil && pluginDetails.Type == protobuff.PluginType_PLUGIN_TYPE_PROJECT {
					cache.RawSchemas.Queries[queryKey] = v
					fmt.Printf("✅ Added plugin query '%s' from %s to project %s\n", k, pluginID, cache.Project.ID)
				} else {
					fmt.Printf("⚠️  Query '%s' from plugin %s already exists in project %s schema\n", k, pluginID, cache.Project.ID)
				}
			}
		}

		// Add mutations to project cache (no locks needed - this is project-specific cache)
		if convertedSchemas.Mutations != nil {
			for k, v := range convertedSchemas.Mutations {
				mutationKey := fmt.Sprintf("%s_%s", PROJECT_PLUGIN_PREFIX, k)
				if cache.RawSchemas.Mutations[mutationKey] == nil && pluginDetails.Type == protobuff.PluginType_PLUGIN_TYPE_PROJECT {
					cache.RawSchemas.Mutations[mutationKey] = v
					fmt.Printf("✅ Added plugin mutation '%s' from %s to project %s\n", k, pluginID, cache.Project.ID)
				} else {
					fmt.Printf("⚠️  Mutation '%s' from plugin %s already exists in project %s schema\n", k, pluginID, cache.Project.ID)
				}
			}
		}
	}

	// Register REST APIs (could be implemented here for API routes)
	// This would integrate with the Echo router for project-specific API endpoints
	// Note: API registration should be carefully designed to avoid locks

	return nil
}

// executePluginGraphQLResolver executes a GraphQL resolver in the plugin using server's cache
func (s *GraphQLServer) executePluginGraphQLResolver(ctx context.Context, pluginID, resolverName, resolverType string, args map[string]interface{}) (interface{}, error) {
	functionType := "graphql_query"
	if resolverType == "mutation" {
		functionType = "graphql_mutation"
	}

	// Try to get plugin from server cache directly (with proper locking)
	plugin := s.tryGetPluginNoBlock(pluginID)
	if plugin == nil {
		return fmt.Sprintf("❌ ERROR: Plugin %s not available", pluginID), nil
	}

	// Get the RPC client
	rpcClient, err := plugin.Client.Client()
	if err != nil {
		return fmt.Sprintf("❌ ERROR: Failed to get RPC client for %s: %v", pluginID, err), nil
	}

	// Dispense the plugin
	raw, err := rpcClient.Dispense(plugin.PluginConfigurations.ExportedVariable)
	if err != nil {
		return fmt.Sprintf("❌ ERROR: Failed to dispense plugin %s: %v", pluginID, err), nil
	}

	loadedPlugin, ok := raw.(*pluginService.HashiCorpNormalPluginGRPC)
	if !ok {
		return fmt.Sprintf("❌ ERROR: Plugin %s is not a valid universal plugin", pluginID), nil
	}

	// Call the actual Execute method

	// Prepare context data to pass sensitive values
	contextData := map[string]interface{}{
		"plugin_id": pluginID,
	}

	// Extract additional context values if needed
	if projectID := ctx.Value("project_id"); projectID != nil {
		contextData["project_id"] = projectID
	}
	if userID := ctx.Value("user_id"); userID != nil {
		contextData["user_id"] = userID
	}
	if tenantID := ctx.Value("tenant_id"); tenantID != nil {
		contextData["tenant_id"] = tenantID
	}

	response, err := loadedPlugin.Execute(ctx, resolverName, functionType, args, contextData)
	if err != nil {
		return fmt.Sprintf("❌ ERROR: Execute failed: %v", err), nil
	}

	if !response.Success {
		return fmt.Sprintf("❌ PLUGIN ERROR: %s", response.Message), nil
	}

	// Parse the result from protobuf Any
	if response.Result != nil {
		// DEBUG: Dump the raw response.Result before unmarshaling
		log.Printf("🔍 [ENGINE-DEBUG] Raw response.Result from plugin %s:", pluginID)
		log.Printf("🔍 [ENGINE-DEBUG] TypeUrl: %s", response.Result.TypeUrl)
		log.Printf("🔍 [ENGINE-DEBUG] Value (bytes): %v", response.Result.Value)
		log.Printf("🔍 [ENGINE-DEBUG] Value (string): %s", string(response.Result.Value))

		// Handle case where JavaScript SDK sends JSON-encoded protobuf data with empty TypeUrl
		if response.Result.TypeUrl == "" && len(response.Result.Value) > 0 {
			log.Printf("🔧 [ENGINE-DEBUG] Detected JSON-encoded protobuf data from JavaScript SDK")

			// Try to parse as JSON
			var jsonData map[string]interface{}
			if err := json.Unmarshal(response.Result.Value, &jsonData); err == nil {
				// Check if it's a ListValue
				if listValue, exists := jsonData["listValue"]; exists {
					log.Printf("🎯 [ENGINE-DEBUG] Detected ListValue in JSON data")
					if listMap, ok := listValue.(map[string]interface{}); ok {
						if values, exists := listMap["values"]; exists {
							if valuesList, ok := values.([]interface{}); ok {
								log.Printf("✅ [ENGINE-DEBUG] Successfully parsed ListValue with %d items", len(valuesList))

								// Convert protobuf structure to Go objects
								convertedList := make([]interface{}, len(valuesList))
								for i, item := range valuesList {
									convertedList[i] = convertProtobufValueToGo(item)
								}

								log.Printf("🔧 [ENGINE-DEBUG] Converted protobuf values to Go objects")
								return convertedList, nil
							}
						}
					}
				}

				// Check if it's a Struct
				if _, exists := jsonData["structValue"]; exists {
					log.Printf("🎯 [ENGINE-DEBUG] Detected StructValue in JSON data")
					convertedStruct := convertProtobufValueToGo(jsonData)
					log.Printf("✅ [ENGINE-DEBUG] Successfully parsed and converted StructValue")
					return convertedStruct, nil
				}

				// Check if it's a simple value
				if stringValue, exists := jsonData["stringValue"]; exists {
					log.Printf("🎯 [ENGINE-DEBUG] Detected StringValue in JSON data")
					return stringValue, nil
				}

				if numberValue, exists := jsonData["numberValue"]; exists {
					log.Printf("🎯 [ENGINE-DEBUG] Detected NumberValue in JSON data")
					return numberValue, nil
				}

				if boolValue, exists := jsonData["boolValue"]; exists {
					log.Printf("🎯 [ENGINE-DEBUG] Detected BoolValue in JSON data")
					return boolValue, nil
				}

				if _, exists := jsonData["nullValue"]; exists {
					log.Printf("🎯 [ENGINE-DEBUG] Detected NullValue in JSON data")
					return nil, nil
				}
			}
		}

		// First try to unmarshal as structpb.Struct (existing approach)
		var resultStruct structpb.Struct
		if err := response.Result.UnmarshalTo(&resultStruct); err == nil {
			resultMap := resultStruct.AsMap()

			// Check if this is JSON-serialized complex data
			if serialization, exists := resultMap["serialization"]; exists && serialization == "json_bytes" {
				log.Printf("🎯 [ENGINE] Detected JSON-serialized complex data from plugin %s", pluginID)
				if data, exists := resultMap["data"]; exists {
					return data, nil
				}
			} else {
				// Regular struct data - check for error conditions first
				if data, exists := resultMap["data"]; exists {
					// Check if the data contains error information
					if dataMap, ok := data.(map[string]interface{}); ok {
						// Check for success field indicating error
						if success, exists := dataMap["success"]; exists {
							if successBool, ok := success.(bool); ok && !successBool {
								// This is an error response, extract error details
								errorMsg := "Unknown error"
								statusCode := 500

								if errMsg, exists := dataMap["error"]; exists {
									if errStr, ok := errMsg.(string); ok {
										errorMsg = errStr
									}
								}

								if code, exists := dataMap["code"]; exists {
									if codeFloat, ok := code.(float64); ok {
										statusCode = int(codeFloat)
									} else if codeInt, ok := code.(int); ok {
										statusCode = codeInt
									}
								}

								// Return a coded error instead of treating as success
								log.Printf("🚨 [ENGINE] Plugin %s returned error: %s (code: %d)", pluginID, errorMsg, statusCode)
								return nil, &CodedError{
									Code:    statusCode,
									Message: errorMsg,
								}
							}
						}
					}

					// No error detected, return data normally
					return data, nil
				}
			}
		}

		// Try to unmarshal as structpb.Value (handles arrays, primitives, etc.)
		var resultValue structpb.Value
		if err := response.Result.UnmarshalTo(&resultValue); err == nil {
			// Handle different value types from structpb.Value
			if stringValue := resultValue.GetStringValue(); stringValue != "" {
				log.Printf("🎯 [ENGINE] Detected JSON string value from plugin %s", pluginID)

				// Try to parse as JSON
				var jsonData map[string]interface{}
				if err := json.Unmarshal([]byte(stringValue), &jsonData); err == nil {
					// Check if this is our JSON-serialized complex data
					if serialization, exists := jsonData["serialization"]; exists && serialization == "json_bytes" {
						log.Printf("✅ [ENGINE] Successfully parsed JSON-serialized complex data from plugin %s", pluginID)
						if data, exists := jsonData["data"]; exists {
							return data, nil
						}
					}
				} else {
					log.Printf("❌ [ENGINE] Failed to parse JSON from plugin %s: %v", pluginID, err)
				}
			} else if listValue := resultValue.GetListValue(); listValue != nil {
				log.Printf("🎯 [ENGINE] Detected list value from plugin %s", pluginID)
				// Convert protobuf ListValue to Go slice
				result := make([]interface{}, len(listValue.Values))
				for i, val := range listValue.Values {
					result[i] = val.AsInterface()
				}
				log.Printf("✅ [ENGINE] Successfully parsed list with %d items from plugin %s", len(result), pluginID)
				return result, nil
			} else if structValue := resultValue.GetStructValue(); structValue != nil {
				log.Printf("🎯 [ENGINE] Detected struct value from plugin %s", pluginID)
				// Convert protobuf Struct to Go map
				result := structValue.AsMap()
				log.Printf("✅ [ENGINE] Successfully parsed struct from plugin %s", pluginID)
				return result, nil
			} else if _, isNumber := resultValue.GetKind().(*structpb.Value_NumberValue); isNumber {
				numberValue := resultValue.GetNumberValue()
				log.Printf("🎯 [ENGINE] Detected number value from plugin %s: %f", pluginID, numberValue)
				return numberValue, nil
			} else if _, isBool := resultValue.GetKind().(*structpb.Value_BoolValue); isBool {
				boolValue := resultValue.GetBoolValue()
				log.Printf("🎯 [ENGINE] Detected boolean value from plugin %s: %t", pluginID, boolValue)
				return boolValue, nil
			} else if _, isNull := resultValue.GetKind().(*structpb.Value_NullValue); isNull {
				log.Printf("🎯 [ENGINE] Detected null value from plugin %s", pluginID)
				return nil, nil
			}
		}

		if err != nil {
			log.Printf("❌ [ENGINE] Error during result parsing for plugin %s: %v", pluginID, err)
		}

		log.Printf("⚠️ [ENGINE] Could not parse result from plugin %s", pluginID)
	}

	return fmt.Sprintf("✅ SUCCESS: Plugin %s.Execute(%s) completed", pluginID, resolverName), nil
}

// ConvertedGraphQLSchemas holds converted GraphQL fields
type ConvertedGraphQLSchemas struct {
	Queries   graphql.Fields
	Mutations graphql.Fields
}

// convertProtoGraphQLSchemaToFieldsWithServer converts protobuf schemas to GraphQL fields with proper server context
func (s *GraphQLServer) convertProtoGraphQLSchemaToFieldsWithServer(protoSchema *protobuff.ThirdPartyGraphQLSchemas, pluginID string) (*ConvertedGraphQLSchemas, error) {
	if protoSchema == nil {
		return &ConvertedGraphQLSchemas{
			Queries:   make(graphql.Fields),
			Mutations: make(graphql.Fields),
		}, nil
	}

	schema := &ConvertedGraphQLSchemas{
		Queries:   make(graphql.Fields),
		Mutations: make(graphql.Fields),
	}

	// First, process object types if they exist
	if protoSchema.Queries != nil {
		queriesMap := protoSchema.Queries.AsMap()
		if objectTypesField, exists := queriesMap["__objectTypes"]; exists {
			if objectTypesMap, ok := objectTypesField.(map[string]interface{}); ok {
				if objectTypesData, exists := objectTypesMap["objectTypes"]; exists {
					if typesMap, ok := objectTypesData.(map[string]interface{}); ok {
						// Store object types for reference resolution
						s.storeObjectTypes(typesMap, pluginID)
					}
				}
			}
		}
	}

	// Convert queries with proper plugin ID injection
	if protoSchema.Queries != nil {
		queriesMap := protoSchema.Queries.AsMap()
		for name, fieldData := range queriesMap {
			// Skip the special __objectTypes field
			if name == "__objectTypes" {
				continue
			}
			if fieldMap, ok := fieldData.(map[string]interface{}); ok {
				// Capture the name and pluginID in the closure properly
				currentName := name
				currentPluginID := pluginID

				// Extract arguments from the plugin schema
				args := s.extractGraphQLArgs(fieldMap)

				schema.Queries[name] = &graphql.Field{
					Type:        s.convertGraphQLTypeFromDataWithContext(s.getTypeFromMap(fieldMap, "type"), pluginID, name),
					Description: s.getStringFromMap(fieldMap, "description"),
					Args:        args,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						// Call the server's method directly with the plugin ID
						return s.executePluginGraphQLResolver(p.Context, currentPluginID, currentName, "query", p.Args)
					},
				}

			}
		}
	}

	// Convert mutations with proper plugin ID injection
	if protoSchema.Mutations != nil {
		mutationsMap := protoSchema.Mutations.AsMap()
		for name, fieldData := range mutationsMap {
			if fieldMap, ok := fieldData.(map[string]interface{}); ok {
				// Capture the name and pluginID in the closure properly
				currentName := name
				currentPluginID := pluginID

				// Extract arguments from the plugin schema
				args := s.extractGraphQLArgs(fieldMap)

				schema.Mutations[name] = &graphql.Field{
					Type:        s.convertGraphQLTypeFromDataWithContext(s.getTypeFromMap(fieldMap, "type"), pluginID, name),
					Description: s.getStringFromMap(fieldMap, "description"),
					Args:        args,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						// Call the server's method directly with the plugin ID
						return s.executePluginGraphQLResolver(p.Context, currentPluginID, currentName, "mutation", p.Args)
					},
				}

			}
		}
	}

	return schema, nil
}

// Helper function for map value extraction (copied from plugins package)
func (s *GraphQLServer) getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// extractGraphQLArgs extracts and converts arguments from plugin field definition
func (s *GraphQLServer) extractGraphQLArgs(fieldMap map[string]interface{}) graphql.FieldConfigArgument {
	args := make(graphql.FieldConfigArgument)

	if argsData, exists := fieldMap["args"]; exists {
		if argsMap, ok := argsData.(map[string]interface{}); ok {
			for argName, argData := range argsMap {
				if argMap, ok := argData.(map[string]interface{}); ok {
					argTypeData := s.getTypeFromMap(argMap, "type")
					argDescription := s.getStringFromMap(argMap, "description")

					// Convert the argument type using the new unified function
					var graphqlType graphql.Type

					// Check if it's a string type that might need properties (backwards compatibility)
					if argTypeStr, ok := argTypeData.(string); ok {
						if argTypeStr == "Object" || argTypeStr == "Object!" || argTypeStr == "[Object]" || argTypeStr == "[Object]!" || argTypeStr == "[Object!]" || argTypeStr == "[Object!]!" {
							// Object type requires properties
							if properties, hasProps := argMap["properties"]; hasProps {
								graphqlType = s.convertGraphQLTypeWithProperties(argTypeStr, properties, argName)
							} else {
								fmt.Printf("[ERROR] extractGraphQLArgs: Object type %s requires 'properties' field for argument %s\n", argTypeStr, argName)
								// Fall back to String if properties are missing
								graphqlType = graphql.String
							}
						} else {
							// Use basic type conversion for non-Object types
							graphqlType = s.convertGraphQLTypeFromData(argTypeData)
						}
					} else {
						// Use the new unified conversion for structured types
						graphqlType = s.convertGraphQLTypeFromData(argTypeData)
					}

					args[argName] = &graphql.ArgumentConfig{
						Type:        graphqlType,
						Description: argDescription,
					}

				}
			}
		}
	}

	return args
}

// convertGraphQLType converts string type representations to GraphQL types
func (s *GraphQLServer) convertGraphQLType(typeStr string) graphql.Type {
	switch typeStr {
	case "String":
		return graphql.String
	case "String!":
		return graphql.NewNonNull(graphql.String)
	case "Int":
		return graphql.Int
	case "Int!":
		return graphql.NewNonNull(graphql.Int)
	case "Float":
		return graphql.Float
	case "Float!":
		return graphql.NewNonNull(graphql.Float)
	case "Boolean":
		return graphql.Boolean
	case "Boolean!":
		return graphql.NewNonNull(graphql.Boolean)
	case "ID":
		return graphql.ID
	case "ID!":
		return graphql.NewNonNull(graphql.ID)
	case "[String]":
		return graphql.NewList(graphql.String)
	case "[String]!":
		return graphql.NewNonNull(graphql.NewList(graphql.String))
	case "[String!]":
		return graphql.NewList(graphql.NewNonNull(graphql.String))
	case "[String!]!":
		return graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String)))
	case "[Int]":
		return graphql.NewList(graphql.Int)
	case "[Int]!":
		return graphql.NewNonNull(graphql.NewList(graphql.Int))
	case "[Int!]":
		return graphql.NewList(graphql.NewNonNull(graphql.Int))
	case "[Int!]!":
		return graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.Int)))
	default:
		// Default to String for unknown types
		return graphql.String
	}
}

// convertGraphQLTypeWithProperties converts string type representations to GraphQL types with dynamic properties
func (s *GraphQLServer) convertGraphQLTypeWithProperties(typeStr string, properties interface{}, argName string) graphql.Type {
	switch typeStr {
	case "Object":
		return s.createDynamicInputType(properties, argName)
	case "Object!":
		return graphql.NewNonNull(s.createDynamicInputType(properties, argName))
	case "[Object]":
		return graphql.NewList(s.createDynamicInputType(properties, argName))
	case "[Object]!":
		return graphql.NewNonNull(graphql.NewList(s.createDynamicInputType(properties, argName)))
	case "[Object!]":
		return graphql.NewList(graphql.NewNonNull(s.createDynamicInputType(properties, argName)))
	case "[Object!]!":
		return graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(s.createDynamicInputType(properties, argName))))
	default:
		// Fall back to basic type conversion
		return s.convertGraphQLType(typeStr)
	}
}

// createDynamicInputType creates a custom input object type based on properties
func (s *GraphQLServer) createDynamicInputType(properties interface{}, argName string) *graphql.InputObject {
	fields := graphql.InputObjectConfigFieldMap{}

	if propsMap, ok := properties.(map[string]interface{}); ok {
		for fieldName, fieldData := range propsMap {
			if fieldMap, ok := fieldData.(map[string]interface{}); ok {
				fieldType := s.getStringFromMap(fieldMap, "type")
				fieldDescription := s.getStringFromMap(fieldMap, "description")

				fields[fieldName] = &graphql.InputObjectFieldConfig{
					Type:        s.convertGraphQLType(fieldType),
					Description: fieldDescription,
				}
			}
		}
	}

	// Generate a unique name based on the fields
	typeName := s.generateInputTypeName(properties, argName)

	return graphql.NewInputObject(graphql.InputObjectConfig{
		Name:        typeName,
		Description: "Dynamic input object type",
		Fields:      fields,
	})
}

// generateInputTypeName generates a unique name for the input type based on its properties
func (s *GraphQLServer) generateInputTypeName(properties interface{}, argName string) string {
	if propsMap, ok := properties.(map[string]interface{}); ok {
		var fieldNames []string
		for fieldName := range propsMap {
			fieldNames = append(fieldNames, fieldName)
		}
		// Sort for consistent naming
		sort.Strings(fieldNames)
		return fmt.Sprintf("DynamicInput_%s_%s", argName, strings.Join(fieldNames, "_"))
	}
	return "DynamicInputObject"
}

// convertProtoGraphQLSchemaToFieldsForSystemSchema converts protobuf schemas to GraphQL fields for system-wide registration
func (s *GraphQLServer) convertProtoGraphQLSchemaToFieldsForSystemSchema(protoSchema *protobuff.ThirdPartyGraphQLSchemas, pluginID string) (*ConvertedGraphQLSchemas, error) {
	if protoSchema == nil {
		return &ConvertedGraphQLSchemas{
			Queries:   make(graphql.Fields),
			Mutations: make(graphql.Fields),
		}, nil
	}

	schema := &ConvertedGraphQLSchemas{
		Queries:   make(graphql.Fields),
		Mutations: make(graphql.Fields),
	}

	// First, process object types if they exist
	if protoSchema.Queries != nil {
		queriesMap := protoSchema.Queries.AsMap()
		if objectTypesField, exists := queriesMap["__objectTypes"]; exists {
			if objectTypesMap, ok := objectTypesField.(map[string]interface{}); ok {
				if objectTypesData, exists := objectTypesMap["objectTypes"]; exists {
					if typesMap, ok := objectTypesData.(map[string]interface{}); ok {
						// Store object types for reference resolution
						s.storeObjectTypes(typesMap, pluginID)
					}
				}
			}
		}
	}

	// Convert queries with proper plugin ID injection for system schema
	if protoSchema.Queries != nil {
		queriesMap := protoSchema.Queries.AsMap()
		for name, fieldData := range queriesMap {
			// Skip the special __objectTypes field
			if name == "__objectTypes" {
				continue
			}

			if fieldMap, ok := fieldData.(map[string]interface{}); ok {
				// Capture the name and pluginID in the closure properly
				currentName := name
				currentPluginID := pluginID

				// Extract arguments from the plugin schema
				args := s.extractGraphQLArgs(fieldMap)

				schema.Queries[name] = &graphql.Field{
					Type:        s.convertGraphQLTypeFromDataWithContext(s.getTypeFromMap(fieldMap, "type"), pluginID, name),
					Description: s.getStringFromMap(fieldMap, "description"),
					Args:        args,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						// Call the server's method directly with the plugin ID for system queries
						return s.executePluginGraphQLResolver(p.Context, currentPluginID, currentName, "query", p.Args)
					},
				}

			}
		}
	}

	// Convert mutations with proper plugin ID injection for system schema
	if protoSchema.Mutations != nil {
		mutationsMap := protoSchema.Mutations.AsMap()
		for name, fieldData := range mutationsMap {
			if fieldMap, ok := fieldData.(map[string]interface{}); ok {
				// Capture the name and pluginID in the closure properly
				currentName := name
				currentPluginID := pluginID

				// Extract arguments from the plugin schema
				args := s.extractGraphQLArgs(fieldMap)

				schema.Mutations[name] = &graphql.Field{
					Type:        s.convertGraphQLTypeFromDataWithContext(s.getTypeFromMap(fieldMap, "type"), pluginID, name),
					Description: s.getStringFromMap(fieldMap, "description"),
					Args:        args,
					Resolve: func(p graphql.ResolveParams) (interface{}, error) {
						// Call the server's method directly with the plugin ID for system mutations
						return s.executePluginGraphQLResolver(p.Context, currentPluginID, currentName, "mutation", p.Args)
					},
				}

			}
		}
	}

	return schema, nil
}

// getTypeFromMap extracts type information from a map, handling both string types and structured GraphQLTypeDefinition objects
func (s *GraphQLServer) getTypeFromMap(m map[string]interface{}, key string) interface{} {
	if val, ok := m[key]; ok {
		return val
	}
	return ""
}

// convertGraphQLTypeFromData converts type data (string or structured) to GraphQL types
func (s *GraphQLServer) convertGraphQLTypeFromData(typeData interface{}) graphql.Type {
	return s.convertGraphQLTypeFromDataWithContext(typeData, "", "")
}

// convertGraphQLTypeFromDataWithContext converts type data with plugin context for unique naming
func (s *GraphQLServer) convertGraphQLTypeFromDataWithContext(typeData interface{}, pluginID, queryName string) graphql.Type {
	// If it's a string, check if it's a scalar type or object type reference
	if typeStr, ok := typeData.(string); ok {
		// Check if it's a scalar type first
		if s.isScalarType(typeStr) {
			return s.convertScalarType(typeStr)
		}

		// Try to resolve as object type reference
		if resolvedType := s.resolveObjectTypeReference(typeStr, pluginID, queryName); resolvedType != graphql.String {
			return resolvedType
		}

		// Fall back to existing convertGraphQLType function
		return s.convertGraphQLType(typeStr)
	}

	// If it's a structured GraphQLTypeDefinition, convert it
	if typeMap, ok := typeData.(map[string]interface{}); ok {
		return s.convertStructuredGraphQLTypeWithContext(typeMap, pluginID, queryName)
	}

	// Default fallback
	return graphql.String
}

// convertGraphQLTypeFromDataWithContextAndField converts type data with plugin context and field name for unique naming
func (s *GraphQLServer) convertGraphQLTypeFromDataWithContextAndField(typeData interface{}, pluginID, queryName, fieldName string) graphql.Type {
	// If it's a string, check if it's a scalar type or object type reference
	if typeStr, ok := typeData.(string); ok {
		// Check if it's a scalar type first
		if s.isScalarType(typeStr) {
			return s.convertScalarType(typeStr)
		}

		// Try to resolve as object type reference with field context
		if resolvedType := s.resolveObjectTypeReferenceWithField(typeStr, pluginID, queryName, fieldName); resolvedType != graphql.String {
			return resolvedType
		}

		// Fall back to existing convertGraphQLType function
		return s.convertGraphQLType(typeStr)
	}

	// If it's a structured GraphQLTypeDefinition, convert it with field context
	if typeMap, ok := typeData.(map[string]interface{}); ok {
		return s.convertStructuredGraphQLTypeWithContextAndField(typeMap, pluginID, queryName, fieldName)
	}

	// Default fallback
	return graphql.String
}

// convertStructuredGraphQLType converts structured GraphQLTypeDefinition to GraphQL types
func (s *GraphQLServer) convertStructuredGraphQLType(typeMap map[string]interface{}) graphql.Type {
	return s.convertStructuredGraphQLTypeWithContext(typeMap, "", "")
}

// convertStructuredGraphQLTypeWithContext converts structured GraphQLTypeDefinition with context
func (s *GraphQLServer) convertStructuredGraphQLTypeWithContext(typeMap map[string]interface{}, pluginID, queryName string) graphql.Type {
	kind := s.getStringFromMap(typeMap, "kind")

	switch kind {
	case "scalar":
		scalarType := s.getStringFromMap(typeMap, "scalarType")
		return s.convertScalarType(scalarType)

	case "object":
		return s.convertObjectType(typeMap, pluginID, queryName)

	case "list":
		if ofType, exists := typeMap["ofType"]; exists {
			if ofTypeMap, ok := ofType.(map[string]interface{}); ok {
				innerType := s.convertStructuredGraphQLTypeWithContext(ofTypeMap, pluginID, queryName)
				return graphql.NewList(innerType)
			}
		}
		return graphql.NewList(graphql.String)

	case "non_null":
		if ofType, exists := typeMap["ofType"]; exists {
			if ofTypeMap, ok := ofType.(map[string]interface{}); ok {
				innerType := s.convertStructuredGraphQLTypeWithContext(ofTypeMap, pluginID, queryName)
				return graphql.NewNonNull(innerType)
			}
		}
		return graphql.NewNonNull(graphql.String)

	default:
		return graphql.String
	}
}

// convertStructuredGraphQLTypeWithContextAndField converts structured GraphQLTypeDefinition with context and field name
func (s *GraphQLServer) convertStructuredGraphQLTypeWithContextAndField(typeMap map[string]interface{}, pluginID, queryName, fieldName string) graphql.Type {
	kind := s.getStringFromMap(typeMap, "kind")

	switch kind {
	case "scalar":
		scalarType := s.getStringFromMap(typeMap, "scalarType")
		return s.convertScalarType(scalarType)

	case "object":
		return s.convertObjectTypeWithContext(typeMap, pluginID, queryName, "field_"+fieldName)

	case "list":
		if ofType, exists := typeMap["ofType"]; exists {
			if ofTypeMap, ok := ofType.(map[string]interface{}); ok {
				innerType := s.convertStructuredGraphQLTypeWithContextAndField(ofTypeMap, pluginID, queryName, fieldName)
				return graphql.NewList(innerType)
			}
		}
		return graphql.NewList(graphql.String)

	case "non_null":
		if ofType, exists := typeMap["ofType"]; exists {
			if ofTypeMap, ok := ofType.(map[string]interface{}); ok {
				innerType := s.convertStructuredGraphQLTypeWithContextAndField(ofTypeMap, pluginID, queryName, fieldName)
				return graphql.NewNonNull(innerType)
			}
		}
		return graphql.NewNonNull(graphql.String)

	default:
		return graphql.String
	}
}

// Store object types globally for reference resolution
var (
	globalObjectTypes      = make(map[string]map[string]interface{})
	globalObjectTypesMutex sync.RWMutex
)

// storeObjectTypes stores object types for later reference resolution
func (s *GraphQLServer) storeObjectTypes(objectTypes map[string]interface{}, pluginID string) {
	globalObjectTypesMutex.Lock()
	defer globalObjectTypesMutex.Unlock()

	for typeName, typeData := range objectTypes {
		if typeMap, ok := typeData.(map[string]interface{}); ok {
			key := fmt.Sprintf("%s_%s", pluginID, typeName)
			globalObjectTypes[key] = typeMap
		}
	}
}

// resolveObjectTypeReference resolves a string type reference to an object type definition
func (s *GraphQLServer) resolveObjectTypeReference(typeName, pluginID, queryName string) graphql.Type {
	// Try to find the object type definition
	key := fmt.Sprintf("%s_%s", pluginID, typeName)

	globalObjectTypesMutex.RLock()
	defer globalObjectTypesMutex.RUnlock()

	if objectTypeDef, exists := globalObjectTypes[key]; exists {
		// Note: We need to copy the map to avoid potential issues when the lock is released
		objectTypeDefCopy := make(map[string]interface{})
		for k, v := range objectTypeDef {
			objectTypeDefCopy[k] = v
		}
		return s.convertObjectTypeFromDefinition(objectTypeDefCopy, pluginID, queryName)
	}

	return graphql.String
}

// resolveObjectTypeReferenceWithField resolves a string type reference with field context for unique naming
func (s *GraphQLServer) resolveObjectTypeReferenceWithField(typeName, pluginID, queryName, fieldName string) graphql.Type {
	// Try to find the object type definition
	key := fmt.Sprintf("%s_%s", pluginID, typeName)

	globalObjectTypesMutex.RLock()
	defer globalObjectTypesMutex.RUnlock()

	if objectTypeDef, exists := globalObjectTypes[key]; exists {
		// Note: We need to copy the map to avoid potential issues when the lock is released
		objectTypeDefCopy := make(map[string]interface{})
		for k, v := range objectTypeDef {
			objectTypeDefCopy[k] = v
		}
		return s.convertObjectTypeWithContext(objectTypeDefCopy, pluginID, queryName, "field_"+fieldName)
	}

	return graphql.String
}

// convertObjectTypeFromDefinition converts an object type definition to GraphQL type
func (s *GraphQLServer) convertObjectTypeFromDefinition(objectTypeDef map[string]interface{}, pluginID, queryName string) graphql.Type {
	typeName := s.getStringFromMap(objectTypeDef, "typeName")
	description := s.getStringFromMap(objectTypeDef, "description")

	// Generate unique name
	uniqueName := s.generateUniqueTypeName(typeName, pluginID, queryName)

	fields := graphql.Fields{}

	if fieldsData, exists := objectTypeDef["fields"]; exists {
		if fieldsMap, ok := fieldsData.(map[string]interface{}); ok {
			for fieldName, fieldData := range fieldsMap {
				if fieldMap, ok := fieldData.(map[string]interface{}); ok {
					fieldType := s.getStringFromMap(fieldMap, "type")
					fieldDescription := s.getStringFromMap(fieldMap, "description")

					// Resolve field type - could be scalar or another object type
					var resolvedType graphql.Type
					if s.isScalarType(fieldType) {
						resolvedType = s.convertScalarType(fieldType)
					} else {
						// Try to resolve as object type reference
						resolvedType = s.resolveObjectTypeReference(fieldType, pluginID, queryName)
					}

					// Capture fieldName in closure to avoid variable scoping issues
					capturedFieldName := fieldName
					fields[fieldName] = &graphql.Field{
						Type:        resolvedType,
						Description: fieldDescription,
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							// Default resolver: extract field value from parent object
							if source, ok := p.Source.(map[string]interface{}); ok {
								if value, exists := source[capturedFieldName]; exists {
									return value, nil
								}
							}
							return nil, nil
						},
					}
				}
			}
		}
	}

	// If no fields, add default
	if len(fields) == 0 {
		fields["__typename"] = &graphql.Field{
			Type:        graphql.String,
			Description: "Type name for object identification",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return uniqueName, nil
			},
		}
	}

	return graphql.NewObject(graphql.ObjectConfig{
		Name:        uniqueName,
		Description: description,
		Fields:      fields,
	})
}

// isScalarType checks if a type name is a scalar type
func (s *GraphQLServer) isScalarType(typeName string) bool {
	switch typeName {
	case "String", "Int", "Boolean", "Float", "ID":
		return true
	default:
		return false
	}
}

// convertScalarType converts scalar type names to GraphQL scalar types
func (s *GraphQLServer) convertScalarType(scalarType string) graphql.Type {
	switch scalarType {
	case "String":
		return graphql.String
	case "Int":
		return graphql.Int
	case "Float":
		return graphql.Float
	case "Boolean":
		return graphql.Boolean
	case "ID":
		return graphql.ID
	default:
		return graphql.String
	}
}

// convertObjectType converts object type definitions to GraphQL object types with unique names
func (s *GraphQLServer) convertObjectType(typeMap map[string]interface{}, pluginID, queryName string) graphql.Type {
	return s.convertObjectTypeWithContext(typeMap, pluginID, queryName, "main")
}

// convertObjectTypeWithContext converts object type definitions with additional context for unique naming
func (s *GraphQLServer) convertObjectTypeWithContext(typeMap map[string]interface{}, pluginID, queryName, context string) graphql.Type {
	baseName := s.getStringFromMap(typeMap, "name")
	if baseName == "" {
		baseName = "DynamicObject"
	}

	// Generate unique name using context as suffix to prevent conflicts
	uniqueName := s.generateUniqueTypeNameWithSuffix(baseName, pluginID, queryName, context)

	fields := graphql.Fields{}

	// Check if this is just a reference (kind:object, name:TypeName) without field definitions
	if kind := s.getStringFromMap(typeMap, "kind"); kind == "object" && typeMap["fields"] == nil {
		// This is a reference to another object type, look it up in the global registry
		key := fmt.Sprintf("%s_%s", pluginID, baseName)

		globalObjectTypesMutex.RLock()
		fullObjectTypeDef, exists := globalObjectTypes[key]
		if exists {
			// Copy the map to avoid potential issues when the lock is released
			fullObjectTypeDefCopy := make(map[string]interface{})
			for k, v := range fullObjectTypeDef {
				fullObjectTypeDefCopy[k] = v
			}
			globalObjectTypesMutex.RUnlock()

			// Use the full object type definition with a "nested" context to make it unique
			return s.convertObjectTypeWithContext(fullObjectTypeDefCopy, pluginID, queryName, "nested_"+baseName)
		}
		globalObjectTypesMutex.RUnlock()
	}

	if fieldsData, exists := typeMap["fields"]; exists {
		if fieldsMap, ok := fieldsData.(map[string]interface{}); ok {
			for fieldName, fieldData := range fieldsMap {
				if fieldMap, ok := fieldData.(map[string]interface{}); ok {
					fieldType := s.getTypeFromMap(fieldMap, "type")
					fieldDescription := s.getStringFromMap(fieldMap, "description")

					// Capture fieldName in closure to avoid variable scoping issues
					capturedFieldName := fieldName
					fields[fieldName] = &graphql.Field{
						Type:        s.convertGraphQLTypeFromDataWithContextAndField(fieldType, pluginID, queryName, fieldName),
						Description: fieldDescription,
						Resolve: func(p graphql.ResolveParams) (interface{}, error) {
							// Default resolver: extract field value from parent object
							if source, ok := p.Source.(map[string]interface{}); ok {
								if value, exists := source[capturedFieldName]; exists {
									return value, nil
								}
							}
							return nil, nil
						},
					}
				}
			}
		}
	}

	// If no fields were found, create a default field to prevent GraphQL errors
	if len(fields) == 0 {
		fields["__typename"] = &graphql.Field{
			Type:        graphql.String,
			Description: "Type name for object identification",
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				return uniqueName, nil
			},
		}
	}

	newType := graphql.NewObject(graphql.ObjectConfig{
		Name:        uniqueName,
		Description: fmt.Sprintf("Dynamic object type: %s (from plugin: %s, query: %s)", baseName, pluginID, queryName),
		Fields:      fields,
	})

	return newType
}

// generateUniqueTypeName generates a unique type name using plugin ID and query name
func (s *GraphQLServer) generateUniqueTypeName(baseName, pluginID, queryName string) string {
	// Clean the plugin ID and query name to make them GraphQL-safe
	cleanPluginID := s.cleanGraphQLName(pluginID)
	cleanQueryName := s.cleanGraphQLName(queryName)

	// Generate unique name: BaseName_PluginID_QueryName
	uniqueName := fmt.Sprintf("%s_%s_%s", baseName, cleanPluginID, cleanQueryName)

	return uniqueName
}

// generateUniqueTypeNameWithSuffix generates a unique type name with an additional suffix for nested objects
func (s *GraphQLServer) generateUniqueTypeNameWithSuffix(baseName, pluginID, queryName, suffix string) string {
	// Clean all components to make them GraphQL-safe
	cleanPluginID := s.cleanGraphQLName(pluginID)
	cleanQueryName := s.cleanGraphQLName(queryName)
	cleanSuffix := s.cleanGraphQLName(suffix)

	// Generate unique name with suffix: BaseName_PluginID_QueryName_Suffix
	uniqueName := fmt.Sprintf("%s_%s_%s_%s", baseName, cleanPluginID, cleanQueryName, cleanSuffix)

	return uniqueName
}

// cleanGraphQLName cleans a string to be GraphQL-safe (alphanumeric + underscore only)
func (s *GraphQLServer) cleanGraphQLName(name string) string {
	// Replace non-alphanumeric characters with underscores
	cleaned := ""
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') {
			cleaned += string(char)
		} else {
			cleaned += "_"
		}
	}

	// Remove leading/trailing underscores and collapse multiple underscores
	cleaned = strings.Trim(cleaned, "_")
	for strings.Contains(cleaned, "__") {
		cleaned = strings.ReplaceAll(cleaned, "__", "_")
	}

	return cleaned
}

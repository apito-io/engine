// Package controller provides HTTP handlers for plugin management operations
package controller

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/apito-io/engine/resolver"
	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/labstack/echo/v4"
)

// PluginV2Controller handles plugin management operations
type PluginV2Controller struct {
	Server *resolver.GraphQLServer
}

// NewPluginV2Controller creates a new plugin v2 controller
func NewPluginV2Controller(server *resolver.GraphQLServer) *PluginV2Controller {
	return &PluginV2Controller{
		Server: server,
	}
}

// Plugin management request/response structures

type PluginOperationResponse struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	PluginID string `json:"plugin_id,omitempty"`
	Status   string `json:"status,omitempty"`
	Error    string `json:"error,omitempty"`
}

type PluginListResponse struct {
	Success bool               `json:"success"`
	Message string             `json:"message"`
	Plugins []PluginStatusInfo `json:"plugins"`
}

type PluginStatusInfo struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Version      string   `json:"version"`
	Status       string   `json:"status"` // loaded, stopped, error
	Language     string   `json:"language"`
	Type         string   `json:"type"`
	Capabilities []string `json:"capabilities"`
	Enable       bool     `json:"enable"`
	Debug        bool     `json:"debug"`
	Official     bool     `json:"official"`
	Publisher    string   `json:"publisher,omitempty"`
	Signed       bool     `json:"signed"`
	BundleURL    string   `json:"bundle_url,omitempty"`
	BundleSHA256 string   `json:"bundle_sha256,omitempty"`
	LastUpdated  string   `json:"last_updated"`
	Error        string   `json:"error,omitempty"`
}

type PluginManifestEntry struct {
	ID              string                       `json:"id"`
	Version         string                       `json:"version"`
	Title           string                       `json:"title"`
	Capabilities    []string                     `json:"capabilities"`
	Activated       bool                         `json:"activated"`
	Official        bool                         `json:"official"`
	Signed          bool                         `json:"signed"`
	Publisher       string                       `json:"publisher,omitempty"`
	BundleURL       string                       `json:"bundle_url,omitempty"`
	BundleSHA256    string                       `json:"bundle_sha256,omitempty"`
	ExtensionPoints []string                     `json:"extension_points"`
	BlockedReason   string                       `json:"blocked_reason,omitempty"`
	Contributions   *pluginService.Contributions `json:"contributions,omitempty"`
}

type PluginManifestResponse struct {
	Success bool                  `json:"success"`
	Message string                `json:"message"`
	Plugins []PluginManifestEntry `json:"plugins"`
}

// CreateOrUpdatePlugin refuses a binary upload. Plugins install from the signed catalog.
func (pc *PluginV2Controller) CreateOrUpdatePlugin(c echo.Context) error {
	if err := pc.requirePluginDeploy(c); err != nil {
		return err
	}
	return c.JSON(http.StatusForbidden, pluginErrorResponse{
		Success: false,
		Code:    "LOCAL_UPLOAD_DISABLED",
		Message: "Plugins are installed from the signed GitHub catalog. Use the marketplace or apito plugin add.",
	})
}

// RestartPlugin restarts a specific plugin
func (pc *PluginV2Controller) RestartPlugin(c echo.Context) error {
	if err := pc.requirePluginDeploy(c); err != nil {
		return err
	}
	pluginID := c.Param("id")
	if pluginID == "" {
		return c.JSON(http.StatusBadRequest, PluginOperationResponse{
			Success: false,
			Message: "Plugin ID is required",
		})
	}

	// Stop the plugin first
	if err := pc.stopPlugin(pluginID); err != nil {
		return c.JSON(http.StatusInternalServerError, PluginOperationResponse{
			Success:  false,
			Message:  "Failed to stop plugin",
			Error:    err.Error(),
			PluginID: pluginID,
		})
	}

	// Start the plugin
	reloadResult, err := pc.hotReloadPlugin(pluginID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, PluginOperationResponse{
			Success:  false,
			Message:  "Failed to restart plugin",
			Error:    err.Error(),
			PluginID: pluginID,
		})
	}

	return c.JSON(http.StatusOK, PluginOperationResponse{
		Success:  true,
		Message:  fmt.Sprintf("Plugin %s %s successfully", pluginID, reloadResult),
		PluginID: pluginID,
		Status:   "loaded",
	})
}

// StopPlugin stops a specific plugin
func (pc *PluginV2Controller) StopPlugin(c echo.Context) error {
	if err := pc.requirePluginDeploy(c); err != nil {
		return err
	}
	pluginID := c.Param("id")
	if pluginID == "" {
		return c.JSON(http.StatusBadRequest, PluginOperationResponse{
			Success: false,
			Message: "Plugin ID is required",
		})
	}

	if err := pc.stopPlugin(pluginID); err != nil {
		return c.JSON(http.StatusInternalServerError, PluginOperationResponse{
			Success:  false,
			Message:  "Failed to stop plugin",
			Error:    err.Error(),
			PluginID: pluginID,
		})
	}

	return c.JSON(http.StatusOK, PluginOperationResponse{
		Success:  true,
		Message:  fmt.Sprintf("Plugin %s stopped successfully", pluginID),
		PluginID: pluginID,
		Status:   "stopped",
	})
}

// DeletePlugin removes a plugin completely
func (pc *PluginV2Controller) DeletePlugin(c echo.Context) error {
	if err := pc.requirePluginDeploy(c); err != nil {
		return err
	}
	pluginID := c.Param("id")
	if pluginID == "" {
		return c.JSON(http.StatusBadRequest, PluginOperationResponse{
			Success: false,
			Message: "Plugin ID is required",
		})
	}

	// Stop the plugin first
	if err := pc.stopPlugin(pluginID); err != nil {
		fmt.Printf("Warning: Failed to stop plugin %s before deletion: %v\n", pluginID, err)
	}

	// Remove plugin directory
	pluginDir := filepath.Join(pc.Server.Cfg.PluginPath, pluginID)
	if err := os.RemoveAll(pluginDir); err != nil {
		return c.JSON(http.StatusInternalServerError, PluginOperationResponse{
			Success:  false,
			Message:  "Failed to remove plugin directory",
			Error:    err.Error(),
			PluginID: pluginID,
		})
	}

	return c.JSON(http.StatusOK, PluginOperationResponse{
		Success:  true,
		Message:  fmt.Sprintf("Plugin %s deleted successfully", pluginID),
		PluginID: pluginID,
		Status:   "deleted",
	})
}

// PlatformInfo represents server platform information
type PlatformInfo struct {
	OS           string `json:"os"`
	Architecture string `json:"architecture"`
	Version      string `json:"version"`
	Hostname     string `json:"hostname"`
}

// PlatformResponse represents the platform API response
type PlatformResponse struct {
	Success  bool         `json:"success"`
	Message  string       `json:"message"`
	Platform PlatformInfo `json:"platform"`
}

// GetPlatformInfo returns the server's platform information for compatibility checking
func (pc *PluginV2Controller) GetPlatformInfo(c echo.Context) error {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	platform := PlatformInfo{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
		Version:      runtime.Version(),
		Hostname:     hostname,
	}

	fmt.Printf("🖥️  [PLATFORM-API] Client requesting platform info: %s/%s\n", platform.OS, platform.Architecture)

	return c.JSON(http.StatusOK, PlatformResponse{
		Success:  true,
		Message:  "Platform information retrieved successfully",
		Platform: platform,
	})
}

// ListPlugins returns the status of all plugins
func (pc *PluginV2Controller) ListPlugins(c echo.Context) error {
	// Load plugin registry
	_hashiCorpPlugins, err := pluginService.LoadHashiCorpPluginRegistry(pc.Server.Cfg)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, PluginListResponse{
			Success: false,
			Message: "Failed to load plugin registry",
		})
	}

	var plugins []PluginStatusInfo
	for pluginID, pluginDetails := range _hashiCorpPlugins {
		status := "stopped"
		errorMsg := ""

		// Check if plugin is currently loaded
		if pluginCache := pc.Server.TryGetPlugin(pluginID); pluginCache != nil {
			if pluginCache.Client != nil && !pluginCache.Client.Exited() {
				status = "loaded"
			} else {
				status = "error"
				errorMsg = "Plugin process exited"
			}
		}

		// Get last modified time of plugin directory
		pluginDir := filepath.Join(pc.Server.Cfg.PluginPath, pluginID)
		var lastUpdated string
		if info, err := os.Stat(pluginDir); err == nil {
			lastUpdated = info.ModTime().Format(time.RFC3339)
		}

		ui := pluginService.UIManifestFor(pluginID)
		plugins = append(plugins, PluginStatusInfo{
			ID:           pluginID,
			Title:        pluginDetails.Title,
			Version:      pluginDetails.Version,
			Status:       status,
			Language:     pluginDetails.Language.String(),
			Type:         "system",
			Capabilities: pluginService.CapabilitiesFor(pluginID),
			Enable:       pluginDetails.Enable,
			Debug:        pluginDetails.Debug,
			Official:     ui.Official,
			Publisher:    ui.Publisher,
			Signed:       ui.Signed,
			BundleURL:    ui.BundleURL,
			BundleSHA256: ui.BundleSHA256,
			LastUpdated:  lastUpdated,
			Error:        errorMsg,
		})
	}

	return c.JSON(http.StatusOK, PluginListResponse{
		Success: true,
		Message: fmt.Sprintf("Found %d plugins", len(plugins)),
		Plugins: plugins,
	})
}

// GetPluginManifest returns authenticated Console UI/extension metadata.
func (pc *PluginV2Controller) GetPluginManifest(c echo.Context) error {
	_hashiCorpPlugins, err := pluginService.LoadHashiCorpPluginRegistry(pc.Server.Cfg)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, PluginManifestResponse{
			Success: false,
			Message: "Failed to load plugin registry",
		})
	}

	projectID := strings.TrimSpace(c.Request().Header.Get("X-Apito-Project-ID"))
	if projectID == "" {
		projectID = c.QueryParam("project_id")
	}
	if v := c.Get("project"); v != nil {
		if s, ok := v.(string); ok && s != "" {
			projectID = s
		}
	}

	activated := map[string]bool{}
	if projectID != "" {
		project, err := pc.Server.LoadProjectCache(c.Request().Context(), projectID)
		if err == nil && project != nil {
			for _, p := range project.Plugins {
				if p == nil {
					continue
				}
				on := p.Enable
				if !on && p.ActivateStatus.String() == "PLUGIN_ACTIVATE_STATUS_ACTIVATED" {
					on = true
				}
				activated[p.ID] = on
			}
		}
	}

	var plugins []PluginManifestEntry
	for pluginID, pluginDetails := range _hashiCorpPlugins {
		caps := pluginService.CapabilitiesFor(pluginID)
		ui := pluginService.UIManifestFor(pluginID)
		entry := PluginManifestEntry{
			ID:              pluginID,
			Version:         pluginDetails.Version,
			Title:           pluginDetails.Title,
			Capabilities:    caps,
			Activated:       activated[pluginID],
			Official:        ui.Official,
			Signed:          ui.Signed,
			Publisher:       ui.Publisher,
			BundleURL:       ui.BundleURL,
			BundleSHA256:    ui.BundleSHA256,
			ExtensionPoints: caps,
			Contributions:   pluginService.ContributionsFor(pluginID),
		}
		if ui.BundleURL != "" && !(ui.Official && ui.Signed && ui.BundleSHA256 != "") {
			entry.BlockedReason = "compiled Console UI requires an Apito-signed official bundle"
			entry.BundleURL = ""
		}
		// A system install is visible on every project. Activated stays false
		// until that project turns the plugin on. Payment and REST stay gated.
		plugins = append(plugins, entry)
	}

	return c.JSON(http.StatusOK, PluginManifestResponse{
		Success: true,
		Message: fmt.Sprintf("Found %d plugins", len(plugins)),
		Plugins: plugins,
	})
}

// GetPluginUI serves the official signed Console bundle extracted next to the plugin binary.
func (pc *PluginV2Controller) GetPluginUI(c echo.Context) error {
	pluginID := strings.TrimSpace(c.Param("id"))
	ui := pluginService.UIManifestFor(pluginID)
	if pluginID == "" || ui.BundleSHA256 == "" {
		return c.NoContent(http.StatusNotFound)
	}
	path, err := pluginService.UIBundlePath(pc.Server.Cfg.PluginPath, pluginID, ui)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}
	sum := sha256.Sum256(body)
	got := hex.EncodeToString(sum[:])
	if !strings.EqualFold(got, ui.BundleSHA256) {
		return c.NoContent(http.StatusConflict)
	}
	c.Response().Header().Set("ETag", `"`+got+`"`)
	c.Response().Header().Set("Cache-Control", "private, max-age=60")
	return c.Blob(http.StatusOK, "application/javascript; charset=utf-8", body)
}

// GetPluginStatus returns the status of a specific plugin
func (pc *PluginV2Controller) GetPluginStatus(c echo.Context) error {
	pluginID := c.Param("id")
	if pluginID == "" {
		return c.JSON(http.StatusBadRequest, PluginOperationResponse{
			Success: false,
			Message: "Plugin ID is required",
		})
	}

	// Load plugin details
	_hashiCorpPlugins, err := pluginService.LoadHashiCorpPluginRegistry(pc.Server.Cfg)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, PluginOperationResponse{
			Success: false,
			Message: "Failed to load plugin registry",
			Error:   err.Error(),
		})
	}

	pluginDetails, exists := _hashiCorpPlugins[pluginID]
	if !exists {
		return c.JSON(http.StatusNotFound, PluginOperationResponse{
			Success:  false,
			Message:  "Plugin not found",
			PluginID: pluginID,
		})
	}

	status := "stopped"
	errorMsg := ""

	// Check if plugin is currently loaded
	if pluginCache := pc.Server.TryGetPlugin(pluginID); pluginCache != nil {
		if pluginCache.Client != nil && !pluginCache.Client.Exited() {
			status = "loaded"
		} else {
			status = "error"
			errorMsg = "Plugin process exited"
		}
	}

	// Get last modified time of plugin directory
	pluginDir := filepath.Join(pc.Server.Cfg.PluginPath, pluginID)
	var lastUpdated string
	if info, err := os.Stat(pluginDir); err == nil {
		lastUpdated = info.ModTime().Format(time.RFC3339)
	}

	pluginInfo := PluginStatusInfo{
		ID:          pluginID,
		Title:       pluginDetails.Title,
		Version:     pluginDetails.Version,
		Status:      status,
		Language:    pluginDetails.Language.String(),
		Type:        pluginDetails.Type.String(),
		Enable:      pluginDetails.Enable,
		Debug:       pluginDetails.Debug,
		LastUpdated: lastUpdated,
		Error:       errorMsg,
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Plugin status retrieved successfully",
		"plugin":  pluginInfo,
	})
}

func (pc *PluginV2Controller) stopPlugin(pluginID string) error {
	pluginCache := pc.Server.TryGetPlugin(pluginID)
	if pluginCache == nil {
		return nil // Plugin is not loaded
	}

	if pluginCache.Client != nil {
		pluginCache.Client.Kill()
	}

	// Remove from cache
	pc.Server.RemoveHashiCorpPlugin(pluginID)

	pc.Server.Lock()
	defer pc.Server.Unlock()

	// Remove from installed plugin list
	var updatedList []string
	for _, id := range pc.Server.InstalledHCPluginList {
		if id != pluginID {
			updatedList = append(updatedList, id)
		}
	}
	pc.Server.InstalledHCPluginList = updatedList

	return nil
}

func (pc *PluginV2Controller) hotReloadPlugin(pluginID string) (string, error) {
	// Check if plugin monitor is available
	if pc.Server.PluginMonitor == nil {
		return "", fmt.Errorf("plugin monitor is not initialized")
	}

	// Check if plugin exists in cache
	plugin := pc.Server.TryGetPlugin(pluginID)

	// If plugin not found in cache, try to load it from registry
	if plugin == nil {
		fmt.Printf("🔍 [PLUGIN-V2] Plugin %s not found in cache, attempting to load from registry...\n", pluginID)

		// Load plugin registry to get configuration
		_hashiCorpPlugins, err := pluginService.LoadHashiCorpPluginRegistry(pc.Server.Cfg)
		if err != nil {
			return "", fmt.Errorf("failed to load plugin registry: %w", err)
		}

		pluginDetails, exists := _hashiCorpPlugins[pluginID]
		if !exists {
			return "", fmt.Errorf("plugin %s not found in registry", pluginID)
		}

		// Check if plugin is enabled before loading
		if !pluginDetails.Enable {
			return "registered_but_disabled", nil
		}

		// Attempt to load the plugin
		pluginDir := filepath.Join(pc.Server.Cfg.PluginPath, pluginID)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fmt.Printf("🚀 [PLUGIN-V2] Loading plugin %s from directory: %s\n", pluginID, pluginDir)

		_, err = pc.Server.LoadHashiCorpPlugin(ctx, pluginDir, pluginDetails)
		if err != nil {
			return "", fmt.Errorf("failed to load plugin %s: %w", pluginID, err)
		}

		// Register plugin with monitor for health tracking
		if pc.Server.PluginMonitor != nil {
			pc.Server.PluginMonitor.RegisterPlugin(pluginID)
		}

		fmt.Printf("✅ [PLUGIN-V2] Successfully loaded plugin %s (version: %s)\n",
			pluginID, pluginDetails.Version)

		return "loaded", nil
	}

	// Plugin exists in cache, check if it's enabled
	if !plugin.PluginConfigurations.Enable {
		return "registered_but_disabled", nil
	}

	// Use plugin monitor's restart function for efficient restart
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("🔄 [PLUGIN-V2] Initiating hot reload for plugin: %s\n", pluginID)

	err := pc.Server.PluginMonitor.RestartPlugin(ctx, pluginID)
	if err != nil {
		return "", fmt.Errorf("failed to restart plugin via monitor: %w", err)
	}

	// Verify the plugin was successfully reloaded
	reloadedPlugin := pc.Server.TryGetPlugin(pluginID)
	if reloadedPlugin == nil {
		return "", fmt.Errorf("plugin %s was not found after restart", pluginID)
	}

	fmt.Printf("🔄 [PLUGIN-V2] Successfully hot reloaded plugin %s (version: %s)\n",
		pluginID, reloadedPlugin.PluginConfigurations.Version)

	return "reloaded", nil
}

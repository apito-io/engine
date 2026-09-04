package controller

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/models"
	pluginService "github.com/apito-io/engine/services"
	pluginPkg "github.com/apito-io/engine/services/plugin"
	"github.com/apito-io/engine/services/plugin/registry"
	"github.com/apito-io/types/protobuff"
	"github.com/labstack/echo/v4"
)

type pluginErrorResponse struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

type pluginCatalogResponse struct {
	Success         bool                    `json:"success"`
	Message         string                  `json:"message"`
	RegistryEnabled bool                    `json:"registry_enabled"`
	CatalogDigest   string                  `json:"catalog_digest,omitempty"`
	RegistryError   string                  `json:"registry_error,omitempty"`
	OS              string                  `json:"os"`
	Arch            string                  `json:"arch"`
	EngineVersion   string                  `json:"engine_version"`
	Plugins         []registry.MergedPlugin `json:"plugins"`
}

type pluginInstallRequest struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Force   bool   `json:"force"`
}

var (
	catalogOnce   sync.Once
	catalogClient *registry.Client
)

func (pc *PluginV2Controller) registryClient() (*registry.Client, error) {
	cfg := pc.Server.Cfg
	var initErr error
	catalogOnce.Do(func() {
		hexKey := strings.TrimSpace(cfg.PluginRegistryPublicKey)
		var pub ed25519.PublicKey
		pub, initErr = registry.ParsePublicKey(hexKey)
		if initErr != nil {
			return
		}
		cacheDir := filepath.Join(cfg.PluginPath, ".catalog")
		catalogClient = &registry.Client{
			URL:       cfg.PluginRegistryURL,
			SigURL:    cfg.PluginRegistrySigURL,
			PublicKey: pub,
			CacheDir:  cacheDir,
		}
	})
	if initErr != nil {
		return nil, initErr
	}
	return catalogClient, nil
}

func (pc *PluginV2Controller) installer() (*registry.Installer, error) {
	client, err := pc.registryClient()
	if err != nil {
		return nil, err
	}
	return &registry.Installer{
		PluginPath:    pc.Server.Cfg.PluginPath,
		EngineVersion: pc.Server.Cfg.EngineVersion,
		Client:        client,
		HotLoad: func(id string) error {
			_, err := pc.hotReloadPlugin(id)
			return err
		},
		Stop:           pc.stopPlugin,
		ActiveProjects: pc.activePluginProjects,
	}, nil
}

func echoIsSuperAdmin(c echo.Context) bool {
	v := c.Get("is_super_admin")
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		return s == "true" || s == "1"
	default:
		return false
	}
}

func actorFromEcho(c echo.Context) string {
	if v := c.Get("user"); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (pc *PluginV2Controller) requirePluginRead(c echo.Context) error {
	if err := pluginService.RequireCapability(c, authz.CapPluginsRead); err != nil {
		return pc.capabilityDenied(c, err)
	}
	return nil
}

func (pc *PluginV2Controller) requirePluginDeploy(c echo.Context) error {
	if err := pluginService.RequireCapability(c, authz.CapPluginsDeploy); err != nil {
		return pc.capabilityDenied(c, err)
	}
	if !echoIsSuperAdmin(c) {
		return c.JSON(http.StatusForbidden, pluginErrorResponse{
			Success: false,
			Code:    registry.CodeForbidden,
			Message: "plugin install/update/uninstall requires a human super-admin session",
		})
	}
	return nil
}

func (pc *PluginV2Controller) capabilityDenied(c echo.Context, err error) error {
	var denied *pluginService.CapabilityDeniedError
	if errors.As(err, &denied) {
		return c.JSON(http.StatusForbidden, pluginErrorResponse{
			Success: false,
			Code:    registry.CodeForbidden,
			Message: denied.Error(),
		})
	}
	return err
}

func registryHTTPStatus(err error) int {
	var re *registry.Error
	if !errors.As(err, &re) {
		return http.StatusInternalServerError
	}
	switch re.Code {
	case registry.CodeNotFound:
		return http.StatusNotFound
	case registry.CodeInUse:
		return http.StatusConflict
	case registry.CodeForbidden, registry.CodeLocalUploadDisabled:
		return http.StatusForbidden
	case registry.CodeRegistryDisabled, registry.CodeRegistryUnavailable, registry.CodeSignatureInvalid:
		return http.StatusServiceUnavailable
	case registry.CodeRollbackCompleted, registry.CodeHealthFailed:
		return http.StatusBadGateway
	default:
		return http.StatusBadRequest
	}
}

func writeRegistryErr(c echo.Context, err error) error {
	var re *registry.Error
	if errors.As(err, &re) {
		return c.JSON(registryHTTPStatus(err), pluginErrorResponse{
			Success: false,
			Code:    re.Code,
			Message: re.Message,
			Error:   re.Error(),
		})
	}
	return c.JSON(http.StatusInternalServerError, pluginErrorResponse{
		Success: false,
		Code:    "INTERNAL",
		Message: err.Error(),
	})
}

func (pc *PluginV2Controller) localInstalled() map[string]registry.InstalledInfo {
	out := map[string]registry.InstalledInfo{}
	plugins, err := pluginPkg.LoadHashiCorpPluginRegistry(pc.Server.Cfg)
	if err != nil {
		return out
	}
	for id, details := range plugins {
		info := registry.InstalledInfo{Version: details.Version, Health: "stopped"}
		if cache := pc.Server.TryGetPlugin(id); cache != nil {
			if cache.Client != nil && !cache.Client.Exited() {
				info.Health = "loaded"
			} else {
				info.Health = "error"
			}
		}
		out[id] = info
	}
	return out
}

func (pc *PluginV2Controller) activePluginProjects(ctx context.Context, pluginID string) ([]string, error) {
	if pc.Server.SystemDriver == nil {
		return nil, nil
	}
	res, err := pc.Server.SystemDriver.SearchProjects(ctx, &models.CommonSystemParams{
		IsSystemRequest: true,
		SkipPagination:  true,
		SkipWhereFilter: true,
	})
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	var ids []string
	for _, p := range res.Results {
		if p == nil {
			continue
		}
		proj := p
		if len(proj.Plugins) == 0 {
			full, gerr := pc.Server.SystemDriver.GetProject(ctx, proj.ID)
			if gerr != nil || full == nil {
				continue
			}
			proj = full
		}
		for _, pl := range proj.Plugins {
			if pl == nil || pl.ID != pluginID {
				continue
			}
			on := pl.Enable
			if !on && pl.ActivateStatus == protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_ACTIVATED {
				on = true
			}
			if on {
				ids = append(ids, proj.ID)
			}
		}
	}
	return ids, nil
}

func (pc *PluginV2Controller) disablePluginOnProjects(ctx context.Context, pluginID string) error {
	ids, err := pc.activePluginProjects(ctx, pluginID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		proj, err := pc.Server.SystemDriver.GetProject(ctx, id)
		if err != nil || proj == nil {
			continue
		}
		for _, pl := range proj.Plugins {
			if pl != nil && pl.ID == pluginID {
				pl.Enable = false
				pl.ActivateStatus = protobuff.PluginActivateStatus_PLUGIN_ACTIVATE_STATUS_DEACTIVATED
			}
		}
		_ = pc.Server.SystemDriver.UpdateProject(ctx, proj, false)
	}
	return nil
}

// GetPluginCatalog returns the signed registry merged with local install state.
func (pc *PluginV2Controller) GetPluginCatalog(c echo.Context) error {
	if err := pc.requirePluginRead(c); err != nil {
		return err
	}
	cfg := pc.Server.Cfg
	resp := pluginCatalogResponse{
		Success:         true,
		RegistryEnabled: cfg.PluginRemoteRegistryEnabled,
		OS:              runtime.GOOS,
		Arch:            runtime.GOARCH,
		EngineVersion:   cfg.EngineVersion,
		Plugins:         []registry.MergedPlugin{},
	}
	installed := pc.localInstalled()
	if !cfg.PluginRemoteRegistryEnabled {
		resp.Message = "remote registry disabled; showing locally installed plugins"
		resp.Plugins = overlayCatalogContributions(registry.Merge(nil, installed, cfg.EngineVersion, runtime.GOOS, runtime.GOARCH))
		return c.JSON(http.StatusOK, resp)
	}
	client, err := pc.registryClient()
	if err != nil {
		return writeRegistryErr(c, err)
	}
	cat, digest, err := client.Get(c.Request().Context())
	if err != nil {
		resp.RegistryError = err.Error()
		resp.Message = "registry unavailable; showing locally installed plugins"
		resp.Plugins = overlayCatalogContributions(registry.Merge(nil, installed, cfg.EngineVersion, runtime.GOOS, runtime.GOARCH))
		return c.JSON(http.StatusOK, resp)
	}
	resp.CatalogDigest = digest
	resp.Message = "ok"
	resp.Plugins = overlayCatalogContributions(registry.Merge(cat, installed, cfg.EngineVersion, runtime.GOOS, runtime.GOARCH))
	return c.JSON(http.StatusOK, resp)
}

// InstallPlugin installs a reviewed catalog plugin onto this Engine.
func (pc *PluginV2Controller) InstallPlugin(c echo.Context) error {
	if err := pc.requirePluginDeploy(c); err != nil {
		return err
	}
	if !pc.Server.Cfg.PluginRemoteRegistryEnabled {
		return c.JSON(http.StatusServiceUnavailable, pluginErrorResponse{
			Success: false,
			Code:    registry.CodeRegistryDisabled,
			Message: "set PLUGIN_REMOTE_REGISTRY_ENABLED=true to install from the signed registry",
		})
	}
	var req pluginInstallRequest
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.ID) == "" {
		return c.JSON(http.StatusBadRequest, pluginErrorResponse{
			Success: false,
			Code:    "BAD_REQUEST",
			Message: "id is required",
		})
	}
	inst, err := pc.installer()
	if err != nil {
		return writeRegistryErr(c, err)
	}
	rec, err := inst.Install(c.Request().Context(), req.ID, req.Version, actorFromEcho(c))
	if err != nil {
		return writeRegistryErr(c, err)
	}
	fmt.Printf("[plugin-registry] installed %s@%s actor=%s digest=%s\n", rec.ID, rec.Version, rec.Actor, rec.CatalogDigest)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("installed %s@%s", rec.ID, rec.Version),
		"record":  rec,
	})
}

// UpdatePlugin installs the currently approved catalog version.
func (pc *PluginV2Controller) UpdatePlugin(c echo.Context) error {
	if err := pc.requirePluginDeploy(c); err != nil {
		return err
	}
	if !pc.Server.Cfg.PluginRemoteRegistryEnabled {
		return c.JSON(http.StatusServiceUnavailable, pluginErrorResponse{
			Success: false,
			Code:    registry.CodeRegistryDisabled,
			Message: "set PLUGIN_REMOTE_REGISTRY_ENABLED=true to update from the signed registry",
		})
	}
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, pluginErrorResponse{Success: false, Code: "BAD_REQUEST", Message: "plugin id required"})
	}
	inst, err := pc.installer()
	if err != nil {
		return writeRegistryErr(c, err)
	}
	rec, err := inst.Update(c.Request().Context(), id, actorFromEcho(c))
	if err != nil {
		return writeRegistryErr(c, err)
	}
	fmt.Printf("[plugin-registry] updated %s@%s actor=%s\n", rec.ID, rec.Version, rec.Actor)
	return c.JSON(http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("updated %s@%s", rec.ID, rec.Version),
		"record":  rec,
	})
}

// UninstallPlugin removes a system-installed plugin.
func (pc *PluginV2Controller) UninstallPlugin(c echo.Context) error {
	if err := pc.requirePluginDeploy(c); err != nil {
		return err
	}
	id := c.Param("id")
	if id == "" {
		return c.JSON(http.StatusBadRequest, pluginErrorResponse{Success: false, Code: "BAD_REQUEST", Message: "plugin id required"})
	}
	force := c.QueryParam("force") == "true" || c.QueryParam("force") == "1"
	var body pluginInstallRequest
	_ = c.Bind(&body)
	force = force || body.Force

	if pc.Server.Cfg.PluginRemoteRegistryEnabled {
		inst, err := pc.installer()
		if err != nil {
			return writeRegistryErr(c, err)
		}
		if force {
			_ = pc.disablePluginOnProjects(c.Request().Context(), id)
		}
		if err := inst.Uninstall(c.Request().Context(), id, force); err != nil {
			return writeRegistryErr(c, err)
		}
		fmt.Printf("[plugin-registry] uninstalled %s force=%v actor=%s\n", id, force, actorFromEcho(c))
		return c.JSON(http.StatusOK, PluginOperationResponse{
			Success:  true,
			Message:  fmt.Sprintf("plugin %s uninstalled", id),
			PluginID: id,
			Status:   "uninstalled",
		})
	}
	return pc.DeletePlugin(c)
}

func overlayCatalogContributions(plugins []registry.MergedPlugin) []registry.MergedPlugin {
	for i := range plugins {
		runtime := pluginPkg.ContributionsFor(plugins[i].ID)
		if runtime == nil {
			continue
		}
		if plugins[i].Contributions == nil || plugins[i].Contributions.Empty() {
			plugins[i].Contributions = runtime
			continue
		}
		plugins[i].Contributions = pluginPkg.MergeContributions(plugins[i].Contributions, nil, runtime)
	}
	return plugins
}

package controller

import (
	"archive/zip"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	pluginService "github.com/apito-io/engine/services/plugin"
	"github.com/labstack/echo/v4"
)

func safePluginSegment(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == ".." {
		return false
	}
	return !strings.ContainsAny(id, `/\`)
}

// DownloadPluginArchive streams the installed plugin directory as a zip.
// Secret env values in config.yml are blank unless include_secrets=true.
func (pc *PluginV2Controller) DownloadPluginArchive(c echo.Context) error {
	if err := pc.requirePluginDeploy(c); err != nil {
		return err
	}
	id := c.Param("id")
	if !safePluginSegment(id) {
		return c.JSON(http.StatusBadRequest, PluginOperationResponse{Success: false, Message: "invalid plugin id"})
	}
	dir := filepath.Join(pc.Server.Cfg.PluginPath, id)
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return c.JSON(http.StatusNotFound, PluginOperationResponse{Success: false, Message: "plugin not installed"})
	}
	includeSecrets := strings.EqualFold(c.QueryParam("include_secrets"), "true")
	res := c.Response()
	res.Header().Set("Content-Type", "application/zip")
	res.Header().Set("Content-Disposition", `attachment; filename="`+id+`.zip"`)
	res.WriteHeader(http.StatusOK)
	zw := zip.NewWriter(res)
	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.Contains(rel, "..") {
			return nil
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !includeSecrets && strings.EqualFold(info.Name(), "config.yml") {
			redacted, redactErr := pluginService.RedactPluginConfigSecrets(raw)
			if redactErr != nil {
				return redactErr
			}
			raw = redacted
		}
		_, err = writer.Write(raw)
		return err
	})
	closeErr := zw.Close()
	if walkErr != nil {
		return walkErr
	}
	return closeErr
}

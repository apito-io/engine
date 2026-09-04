package registry

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Installer performs verified install/update/uninstall/rollback.
type Installer struct {
	PluginPath    string
	EngineVersion string
	Client        *Client
	HTTP          *http.Client
	MaxBytes      int64
	GOOS          string
	GOARCH        string

	HotLoad func(id string) error
	Stop    func(id string) error
	// ActiveProjects returns project IDs that still have this plugin enabled.
	ActiveProjects func(ctx context.Context, pluginID string) ([]string, error)
}

func (in *Installer) goos() string {
	if in.GOOS != "" {
		return in.GOOS
	}
	return runtime.GOOS
}

func (in *Installer) goarch() string {
	if in.GOARCH != "" {
		return in.GOARCH
	}
	return runtime.GOARCH
}

func (in *Installer) http() *http.Client {
	if in.HTTP != nil {
		return in.HTTP
	}
	return &http.Client{Timeout: 60 * time.Second, CheckRedirect: boundedRedirects}
}

func installable(e *CatalogEntry) error {
	switch strings.ToLower(e.Status) {
	case "blocked", "catalog-stub":
		return coded(CodeNotInstallable, "plugin status is "+e.Status, nil)
	case "deprecated":
		return coded(CodeNotInstallable, "plugin is deprecated", nil)
	}
	if strings.ToLower(e.Trust) != "official" && strings.ToLower(e.Trust) != "community" {
		return coded(CodeNotInstallable, "unknown trust "+e.Trust, nil)
	}
	if len(e.Releases) == 0 {
		return coded(CodeNotInstallable, "no reviewed release assets", nil)
	}
	return nil
}

// Install downloads and atomically promotes a catalog plugin.
func (in *Installer) Install(ctx context.Context, id, version, actor string) (*InstallRecord, error) {
	cat, digest, err := in.Client.Get(ctx)
	if err != nil {
		return nil, err
	}
	entry := cat.Find(id)
	if entry == nil {
		return nil, coded(CodeNotFound, "plugin not in signed catalog", nil)
	}
	if version != "" && version != entry.PluginVersion && version != entry.Runtime.Version {
		return nil, coded(CodeNotFound, "requested version is not the approved catalog version", nil)
	}
	if err := installable(entry); err != nil {
		return nil, err
	}
	if !CompatibleEngine(in.EngineVersion, entry.EngineSemver) {
		return nil, coded(CodeIncompatibleEngine, fmt.Sprintf("engine %s does not satisfy %s", in.EngineVersion, entry.EngineSemver), nil)
	}
	rel := entry.SelectRelease(in.goos(), in.goarch())
	if rel == nil {
		return nil, coded(CodeUnsupportedPlatform, fmt.Sprintf("no asset for %s/%s", in.goos(), in.goarch()), nil)
	}
	if rel.SHA256 == "" || rel.URL == "" {
		return nil, coded(CodeNotInstallable, "release is missing url or sha256", nil)
	}

	stagingRoot := filepath.Join(in.PluginPath, ".staging", id)
	_ = os.RemoveAll(stagingRoot)
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(stagingRoot)

	zipPath := filepath.Join(stagingRoot, "plugin.zip")
	extractDir := filepath.Join(stagingRoot, "extract")
	if err := DownloadVerified(ctx, in.http(), rel.URL, rel.Size, rel.SHA256, zipPath, in.MaxBytes); err != nil {
		return nil, err
	}
	if err := SafeExtractZip(zipPath, extractDir, entry.Runtime.BinaryPath, 0); err != nil {
		return nil, err
	}
	if err := matchRuntime(filepath.Join(extractDir, "config.yml"), entry.Runtime); err != nil {
		return nil, err
	}

	dest := filepath.Join(in.PluginPath, id)
	prev := filepath.Join(in.PluginPath, ".previous", id)
	var previousVersion string
	if _, err := os.Stat(dest); err == nil {
		if rec, recErr := ReadInstallRecord(in.PluginPath, id); recErr == nil && rec != nil {
			previousVersion = rec.Version
		}
		_ = os.RemoveAll(prev)
		if err := os.MkdirAll(filepath.Dir(prev), 0o755); err != nil {
			return nil, err
		}
		if err := os.Rename(dest, prev); err != nil {
			return nil, err
		}
	}

	if in.Stop != nil {
		_ = in.Stop(id)
	}
	if err := os.Rename(extractDir, dest); err != nil {
		if _, prevErr := os.Stat(prev); prevErr == nil {
			_ = os.Rename(prev, dest)
		}
		return nil, fmt.Errorf("promote plugin directory: %w", err)
	}

	rec := InstallRecord{
		ID:              id,
		Version:         entry.PluginVersion,
		CatalogDigest:   digest,
		ArtifactSHA256:  rel.SHA256,
		OS:              rel.OS,
		Arch:            rel.Arch,
		Actor:           actor,
		InstalledAt:     time.Now().UTC(),
		State:           "installed",
		PreviousVersion: previousVersion,
		SourceURL:       rel.URL,
	}
	if err := writeInstallRecord(in.PluginPath, rec); err != nil {
		return nil, err
	}
	if in.HotLoad != nil {
		if err := in.HotLoad(id); err != nil {
			if rbErr := in.Rollback(ctx, id); rbErr != nil {
				return nil, coded(CodeHealthFailed, "plugin failed health check and rollback failed", err)
			}
			return nil, coded(CodeRollbackCompleted, "plugin failed health check; previous version restored", err)
		}
	}
	return &rec, nil
}

// Update installs the currently approved catalog version.
func (in *Installer) Update(ctx context.Context, id, actor string) (*InstallRecord, error) {
	return in.Install(ctx, id, "", actor)
}

// Uninstall removes a plugin. force skips in-use protection.
func (in *Installer) Uninstall(ctx context.Context, id string, force bool) error {
	if in.ActiveProjects != nil && !force {
		ids, err := in.ActiveProjects(ctx, id)
		if err != nil {
			return err
		}
		if len(ids) > 0 {
			return coded(CodeInUse, fmt.Sprintf("plugin is active in %d project(s); pass force to uninstall", len(ids)), nil)
		}
	}
	if in.Stop != nil {
		_ = in.Stop(id)
	}
	dest := filepath.Join(in.PluginPath, id)
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	_ = os.RemoveAll(filepath.Join(in.PluginPath, ".previous", id))
	if rec, err := ReadInstallRecord(in.PluginPath, id); err == nil && rec != nil {
		rec.State = "uninstalled"
		rec.InstalledAt = time.Now().UTC()
		_ = writeInstallRecord(in.PluginPath, *rec)
	}
	return nil
}

// Rollback restores .previous/<id> if present.
func (in *Installer) Rollback(ctx context.Context, id string) error {
	prev := filepath.Join(in.PluginPath, ".previous", id)
	dest := filepath.Join(in.PluginPath, id)
	if _, err := os.Stat(prev); err != nil {
		return coded(CodeNotFound, "no previous version to restore", err)
	}
	if in.Stop != nil {
		_ = in.Stop(id)
	}
	failed := dest + ".failed"
	_ = os.RemoveAll(failed)
	if _, err := os.Stat(dest); err == nil {
		_ = os.Rename(dest, failed)
	}
	if err := os.Rename(prev, dest); err != nil {
		return err
	}
	_ = os.RemoveAll(failed)
	if rec, err := ReadInstallRecord(in.PluginPath, id); err == nil && rec != nil {
		rec.State = "rolled_back"
		rec.Version, rec.PreviousVersion = rec.PreviousVersion, rec.Version
		_ = writeInstallRecord(in.PluginPath, *rec)
	}
	if in.HotLoad != nil {
		return in.HotLoad(id)
	}
	return nil
}

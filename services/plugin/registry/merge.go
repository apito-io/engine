package registry

import (
	"runtime"
	"strings"
)

// InstalledInfo is local host state for a plugin id.
type InstalledInfo struct {
	Version string
	Health  string
}

// Merge overlays signed catalog metadata with local install/health.
func Merge(cat *Catalog, installed map[string]InstalledInfo, engineVer, goos, goarch string) []MergedPlugin {
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	if installed == nil {
		installed = map[string]InstalledInfo{}
	}
	seen := map[string]struct{}{}
	var out []MergedPlugin
	if cat != nil {
		for _, e := range cat.Plugins {
			m := mergeOne(e, installed[e.ID], engineVer, goos, goarch)
			out = append(out, m)
			seen[e.ID] = struct{}{}
		}
	}
	for id, info := range installed {
		if _, ok := seen[id]; ok {
			continue
		}
		out = append(out, MergedPlugin{
			CatalogEntry: CatalogEntry{
				ID:            id,
				Name:          id,
				PluginVersion: info.Version,
				Status:        "installed-local",
				Trust:         "community",
			},
			Installed:         true,
			InstalledVersion:  info.Version,
			Health:            info.Health,
			Compatible:        true,
			PlatformSupported: true,
		})
	}
	return out
}

func mergeOne(e CatalogEntry, info InstalledInfo, engineVer, goos, goarch string) MergedPlugin {
	m := MergedPlugin{
		CatalogEntry:      e,
		Compatible:        CompatibleEngine(engineVer, e.EngineSemver),
		PlatformSupported: e.SelectRelease(goos, goarch) != nil,
	}
	if info.Version != "" || info.Health != "" {
		m.Installed = true
		m.InstalledVersion = info.Version
		m.Health = info.Health
		m.UpdateAvailable = VersionNewer(e.PluginVersion, info.Version)
	}
	switch strings.ToLower(e.Status) {
	case "blocked":
		m.BlockedReason = "plugin is blocked in the registry"
		m.PlatformSupported = false
	case "catalog-stub":
		m.BlockedReason = "catalog stub; no reviewed release"
		m.PlatformSupported = false
	}
	if !m.Compatible {
		m.BlockedReason = "incompatible with this Engine version"
	}
	if m.Compatible && !m.PlatformSupported && e.Status == "available" && len(e.Releases) > 0 {
		m.BlockedReason = "no artifact for this host OS/architecture"
	}
	return m
}

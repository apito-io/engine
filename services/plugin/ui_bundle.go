package plugin

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultUIEntry = "ui.js"

// SealUIBundle hashes the extracted UI file and points Console at Engine, never GitHub.
func SealUIBundle(id, pluginDir string, cfg *YAMLUIConfig) UIManifest {
	m := UIManifest{}
	if cfg == nil {
		return m
	}
	m.Official = cfg.Official
	m.Signed = cfg.Signed
	m.Publisher = strings.TrimSpace(cfg.Publisher)
	m.EntryPath = sanitizeUIEntry(cfg.EntryPath)
	if m.EntryPath == "" {
		m.EntryPath = defaultUIEntry
	}
	id = strings.TrimSpace(id)
	body, err := os.ReadFile(filepath.Join(pluginDir, m.EntryPath))
	if err != nil || len(body) == 0 || id == "" {
		return m
	}
	sum := sha256.Sum256(body)
	m.BundleSHA256 = hex.EncodeToString(sum[:])
	m.BundleURL = fmt.Sprintf("/system/plugin/%s/ui.js", id)
	return m
}

// UIBundlePath is the on-disk UI file for an installed plugin.
func UIBundlePath(pluginRoot, id string, ui UIManifest) (string, error) {
	id = strings.TrimSpace(id)
	entry := sanitizeUIEntry(ui.EntryPath)
	if id == "" || entry == "" {
		return "", fmt.Errorf("missing plugin ui")
	}
	return filepath.Join(pluginRoot, id, entry), nil
}

func sanitizeUIEntry(raw string) string {
	name := filepath.Base(strings.TrimSpace(raw))
	if name == "" || name == "." || name == string(filepath.Separator) {
		return ""
	}
	if strings.Contains(name, "..") {
		return ""
	}
	return name
}

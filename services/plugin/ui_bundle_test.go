package plugin

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
)

func TestSealUIBundleHashesUiJS(t *testing.T) {
	id := "hc-ui-bundle-plugin"
	t.Cleanup(func() { SetPluginUIManifest(id, UIManifest{}) })

	dir := t.TempDir()
	pluginDir := filepath.Join(dir, id)
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	js := []byte("window.ApitoPluginSDK.register({name:'x'});")
	if err := os.WriteFile(filepath.Join(pluginDir, "ui.js"), js, 0o644); err != nil {
		t.Fatal(err)
	}

	yml := []byte(`plugin:
  id: hc-ui-bundle-plugin
  language: go
  title: UI Bundle
  capabilities:
    - console.routes
  enable: true
  binary_path: hc-ui-bundle-plugin
  handshake_config:
    protocol_version: 1
    magic_cookie_key: APITO_PLUGIN
    magic_cookie_value: apito_plugin_magic_cookie_v1
  ui_config:
    entry_path: ui.js
    official: true
    signed: true
    publisher: Apito
`)
	if err := os.WriteFile(filepath.Join(pluginDir, "config.yml"), yml, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadHashiCorpPluginRegistryFromYAML(&models.Config{PluginPath: dir}); err != nil {
		t.Fatal(err)
	}
	ui := UIManifestFor(id)
	if !ui.Official || !ui.Signed || ui.Publisher != "Apito" {
		t.Fatalf("manifest flags=%+v", ui)
	}
	if ui.BundleURL != "/system/plugin/"+id+"/ui.js" {
		t.Fatalf("bundle_url=%q", ui.BundleURL)
	}
	if ui.BundleSHA256 == "" || ui.EntryPath != "ui.js" {
		t.Fatalf("hash/entry=%+v", ui)
	}
	path, err := UIBundlePath(dir, id, ui)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, filepath.Join(id, "ui.js")) {
		t.Fatalf("path=%q", path)
	}
}

func TestSealUIBundleRejectsTraversalEntry(t *testing.T) {
	got := sanitizeUIEntry("../secret.js")
	if got != "secret.js" {
		t.Fatalf("got %q", got)
	}
	if sanitizeUIEntry("") != "" {
		t.Fatal("empty should stay empty")
	}
}

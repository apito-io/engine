package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStampYAMLVersion(t *testing.T) {
	in := []byte("plugin:\n  id: hc-x\n  version: \"0.0.3\"\n  binary_path: hc-x\n")
	got := string(stampYAMLVersion(in, "0.0.4"))
	if !strings.Contains(got, `version: "0.0.4"`) {
		t.Fatalf("got %q", got)
	}
}

func TestMatchRuntimeStampsCatalogVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yml")
	raw := []byte(`plugin:
  id: "hc-cloudinary-plugin"
  version: "0.0.3"
  binary_path: "hc-cloudinary-plugin"
  capabilities:
    - project.graphql
  handshake_config:
    protocol_version: 1
    magic_cookie_key: "APITO_PLUGIN"
    magic_cookie_value: "apito_plugin_magic_cookie_v1"
`)
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	want := RuntimeContract{
		ID: "hc-cloudinary-plugin", Version: "0.0.4", BinaryPath: "hc-cloudinary-plugin",
		Capabilities: []string{"project.graphql"},
		Handshake: Handshake{ProtocolVersion: 1, MagicCookieKey: "APITO_PLUGIN", MagicCookieValue: "apito_plugin_magic_cookie_v1"},
	}
	if err := matchRuntime(path, want); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `version: "0.0.4"`) {
		t.Fatalf("not stamped: %s", b)
	}
}

package plugin

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/types/protobuff"
)

func TestConvertYAMLPluginCapabilities(t *testing.T) {
	got, err := convertYAMLPluginToProtobuf(YAMLPlugin{
		ID:           "hc-hello-plugin",
		Language:     "go",
		Capabilities: []string{CapProjectGraphQL, CapProjectREST},
		Enable:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != protobuff.PluginType_PLUGIN_TYPE_SYSTEM {
		t.Fatalf("type=%v", got.Type)
	}
	if !HasCapability("hc-hello-plugin", CapProjectGraphQL) {
		t.Fatal("missing project.graphql")
	}
	t.Cleanup(func() { SetPluginCapabilities("hc-hello-plugin", nil) })
}

func TestConvertYAMLPluginLegacyProjectType(t *testing.T) {
	t.Setenv("PLUGIN_STRICT_TYPE", "")
	t.Setenv("ENVIRONMENT", "production")
	got, err := convertYAMLPluginToProtobuf(YAMLPlugin{
		ID:       "hc-legacy-plugin",
		Language: "go",
		Type:     "project",
		Enable:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != protobuff.PluginType_PLUGIN_TYPE_SYSTEM {
		t.Fatalf("type=%v", got.Type)
	}
	if !HasCapability("hc-legacy-plugin", CapProjectREST) {
		t.Fatal("legacy project type should map to project.rest")
	}
	t.Cleanup(func() { SetPluginCapabilities("hc-legacy-plugin", nil) })
}

func TestConvertYAMLPluginLegacyProjectTypeRejectedInDev(t *testing.T) {
	t.Setenv("PLUGIN_STRICT_TYPE", "1")
	_, err := convertYAMLPluginToProtobuf(YAMLPlugin{
		ID:       "hc-legacy-dev-plugin",
		Language: "go",
		Type:     "project",
	})
	if err == nil {
		t.Fatal("expected migration error")
	}
}

func TestLoadHashiCorpPluginRegistryFromYAML(t *testing.T) {
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "hc-yaml-plugin")
	if err := os.MkdirAll(pluginDir, 0o755); err != nil {
		t.Fatal(err)
	}
	yml := []byte(`plugin:
  id: hc-yaml-plugin
  language: go
  title: YAML Test
  capabilities:
    - system.graphql
    - system.rest
  enable: true
  binary_path: hc-yaml-plugin
  handshake_config:
    protocol_version: 1
    magic_cookie_key: APITO_PLUGIN
    magic_cookie_value: apito_plugin_magic_cookie_v1
`)
	if err := os.WriteFile(filepath.Join(pluginDir, "config.yml"), yml, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadHashiCorpPluginRegistryFromYAML(&models.Config{PluginPath: dir})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := got["hc-yaml-plugin"]; !ok {
		t.Fatalf("missing plugin: %#v", got)
	}
	if !HasCapability("hc-yaml-plugin", CapSystemGraphQL) {
		t.Fatal("missing system.graphql")
	}
	t.Cleanup(func() { SetPluginCapabilities("hc-yaml-plugin", nil) })
}

// Package plugin provides functionality for loading and managing HashiCorp plugins
// from individual config.yml files in plugin directories.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types/protobuff"
	"gopkg.in/yaml.v3"
)

// YAMLPluginConfig represents the structure of the individual plugin config.yml file
type YAMLPluginConfig struct {
	Plugin YAMLPlugin `yaml:"plugin"`
}

// YAMLUIConfig represents the UI configuration in YAML
type YAMLUIConfig struct {
	EntryPath    string `yaml:"entry_path"`
	Official     bool   `yaml:"official"`
	BundleURL    string `yaml:"bundle_url"`
	BundleSHA256 string `yaml:"bundle_sha256"`
	Publisher    string `yaml:"publisher"`
	Signed       bool   `yaml:"signed"`
}

// YAMLPlugin represents a single plugin configuration in YAML
type YAMLPlugin struct {
	ID               string              `yaml:"id"`
	Language         string              `yaml:"language"`
	Title            string              `yaml:"title"`
	Icon             string              `yaml:"icon"`
	Description      string              `yaml:"description"`
	Type             string              `yaml:"type"` // deprecated; use capabilities
	Capabilities     []string            `yaml:"capabilities"`
	Role             string              `yaml:"role"`
	ExportedVariable string              `yaml:"exported_variable"`
	Enable           bool                `yaml:"enable"`
	Debug            bool                `yaml:"debug"`
	Version          string              `yaml:"version"`
	Author           string              `yaml:"author"`
	RepositoryURL    string              `yaml:"repository_url"`
	Branch           string              `yaml:"branch"`
	BinaryPath       string              `yaml:"binary_path"`
	HandshakeConfig  YAMLHandshakeConfig `yaml:"handshake_config"`
	UIConfig         *YAMLUIConfig       `yaml:"ui_config,omitempty"`
	Contributions    *Contributions      `yaml:"contributions,omitempty"`
	EnvVars          []YAMLEnvVar        `yaml:"env_vars"`
}

// YAMLHandshakeConfig represents the handshake configuration in YAML
type YAMLHandshakeConfig struct {
	ProtocolVersion  int32  `yaml:"protocol_version"`
	MagicCookieKey   string `yaml:"magic_cookie_key"`
	MagicCookieValue string `yaml:"magic_cookie_value"`
}

// YAMLEnvVar represents an environment variable in YAML
type YAMLEnvVar struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

// LoadHashiCorpPluginRegistryFromYAML loads plugin configurations from individual config.yml files
func LoadHashiCorpPluginRegistryFromYAML(cfg *models.Config) (map[string]*protobuff.PluginDetails, error) {
	plugins := make(map[string]*protobuff.PluginDetails)

	// Read the plugin directory
	entries, err := os.ReadDir(cfg.PluginPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin directory %s: %w", cfg.PluginPath, err)
	}

	// Scan each subdirectory for config.yml files
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Skip files, only process directories
		}

		pluginDir := entry.Name()
		configPath := filepath.Join(cfg.PluginPath, pluginDir, "config.yml")

		// Check if config.yml exists in this plugin directory
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			// No config.yml found, skip this directory (not a plugin)
			continue
		}

		// Read the config.yml file
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config.yml in plugin %s: %w", pluginDir, err)
		}

		// Parse the individual plugin config
		var config YAMLPluginConfig
		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config.yml in plugin %s: %w", pluginDir, err)
		}

		// Convert YAML config to protobuf PluginDetails
		pluginDetails, err := convertYAMLPluginToProtobuf(config.Plugin)
		if err != nil {
			return nil, fmt.Errorf("failed to convert plugin %s: %w", pluginDir, err)
		}
		if config.Plugin.UIConfig != nil {
			SetPluginUIManifest(config.Plugin.ID, SealUIBundle(
				config.Plugin.ID,
				filepath.Join(cfg.PluginPath, pluginDir),
				config.Plugin.UIConfig,
			))
		}

		// Use plugin ID from config as the key
		plugins[config.Plugin.ID] = pluginDetails
	}

	return plugins, nil
}

// convertYAMLPluginToProtobuf converts a YAMLPlugin to protobuff.PluginDetails
func convertYAMLPluginToProtobuf(yamlPlugin YAMLPlugin) (*protobuff.PluginDetails, error) {
	// Convert language string to enum
	language, err := convertLanguageStringToEnum(yamlPlugin.Language)
	if err != nil {
		return nil, err
	}

	if err := RejectLegacyProjectTypeInDev(yamlPlugin.Type, yamlPlugin.Capabilities); err != nil {
		return nil, fmt.Errorf("plugin %s: %w", yamlPlugin.ID, err)
	}

	caps := NormalizeCapabilities(yamlPlugin.Capabilities)
	if len(caps) == 0 {
		caps, err = CapabilitiesFromLegacyType(yamlPlugin.Type)
		if err != nil {
			return nil, fmt.Errorf("plugin %s: %w", yamlPlugin.ID, err)
		}
		if strings.EqualFold(yamlPlugin.Type, "project") || strings.EqualFold(yamlPlugin.Type, "external") {
			fmt.Printf("plugin %s: type: project is deprecated; treating as capabilities %v\n", yamlPlugin.ID, caps)
		}
	}
	SetPluginCapabilities(yamlPlugin.ID, caps)
	if yamlPlugin.UIConfig != nil {
		SetPluginUIManifest(yamlPlugin.ID, UIManifest{
			EntryPath:    yamlPlugin.UIConfig.EntryPath,
			Official:     yamlPlugin.UIConfig.Official,
			BundleURL:    yamlPlugin.UIConfig.BundleURL,
			BundleSHA256: yamlPlugin.UIConfig.BundleSHA256,
			Publisher:    yamlPlugin.UIConfig.Publisher,
			Signed:       yamlPlugin.UIConfig.Signed,
		})
	}
	SetPluginContributions(yamlPlugin.ID, NormalizeContributions(yamlPlugin.Contributions))

	// Convert environment variables
	var envVars []*protobuff.EnvVariable
	for _, envVar := range yamlPlugin.EnvVars {
		// Handle environment variable substitution
		value := envVar.Value
		if value == "" && envVar.Key == "PROJECT_DB_DATABASE" {
			value = utility.GetEnv("PROJECT_DB_DATABASE", "")
		}

		envVars = append(envVars, &protobuff.EnvVariable{
			Key:   envVar.Key,
			Value: value,
		})
	}

	// Set default exported variable based on plugin type if not specified
	exportedVariable := yamlPlugin.ExportedVariable
	if exportedVariable == "" {
		exportedVariable = _const.NormalPluginRPCName
	}

	return &protobuff.PluginDetails{
		Id:               yamlPlugin.ID,
		Language:         language,
		Title:            yamlPlugin.Title,
		Icon:             yamlPlugin.Icon,
		Description:      yamlPlugin.Description,
		Type:             protobuff.PluginType_PLUGIN_TYPE_SYSTEM,
		Role:             yamlPlugin.Role,
		ExportedVariable: exportedVariable,
		Enable:           yamlPlugin.Enable,
		Debug:            yamlPlugin.Debug,
		Version:          yamlPlugin.Version,
		Author:           yamlPlugin.Author,
		RepositoryUrl:    yamlPlugin.RepositoryURL,
		Branch:           yamlPlugin.Branch,
		BinaryPath:       yamlPlugin.BinaryPath,
		HandshakeConfig: &protobuff.HashiCorpHandshakeConfig{
			ProtocolVersion:  uint32(yamlPlugin.HandshakeConfig.ProtocolVersion),
			MagicCookieKey:   yamlPlugin.HandshakeConfig.MagicCookieKey,
			MagicCookieValue: yamlPlugin.HandshakeConfig.MagicCookieValue,
		},
		EnvVars: envVars,
	}, nil
}

// convertLanguageStringToEnum converts language string to protobuff enum
func convertLanguageStringToEnum(language string) (protobuff.PluginLanguage, error) {
	switch strings.ToLower(language) {
	case "go", "golang":
		return protobuff.PluginLanguage_PLUGIN_LANGUAGE_GO, nil
	case "js", "javascript":
		return protobuff.PluginLanguage_PLUGIN_LANGUAGE_JS, nil
	case "cpp", "c++":
		return protobuff.PluginLanguage_PLUGIN_LANGUAGE_CPP, nil
	case "python", "py":
		return protobuff.PluginLanguage_PLUGIN_LANGUAGE_PYTHON, nil
	case "java":
		return protobuff.PluginLanguage_PLUGIN_LANGUAGE_JAVA, nil
	case "ruby", "rb":
		return protobuff.PluginLanguage_PLUGIN_LANGUAGE_RUBY, nil
	case "php":
		return protobuff.PluginLanguage_PLUGIN_LANGUAGE_PHP, nil
	case "csharp", "c#":
		return protobuff.PluginLanguage_PLUGIN_LANGUAGE_CSHARP, nil
	case "typescript", "ts":
		return protobuff.PluginLanguage_PLUGIN_LANGUAGE_TYPESCRIPT, nil
	default:
		return protobuff.PluginLanguage_PLUGIN_LANGUAGE_GO, fmt.Errorf("unsupported plugin language: %s", language)
	}
}

// convertTypeStringToEnum is retained for CLI/docs callers. All plugins are system-installed.
func convertTypeStringToEnum(pluginType string) (protobuff.PluginType, error) {
	switch strings.ToLower(pluginType) {
	case "", "system", "internal", "project", "external":
		return protobuff.PluginType_PLUGIN_TYPE_SYSTEM, nil
	default:
		return protobuff.PluginType_PLUGIN_TYPE_SYSTEM, fmt.Errorf("unsupported plugin type: %s; plugins are system-installed", pluginType)
	}
}

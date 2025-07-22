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

// YAMLPluginConfig represents the structure of the plugins.yml file
type YAMLPluginConfig struct {
	Plugins map[string]YAMLPlugin `yaml:"plugins"`
}

// YAMLPlugin represents a single plugin configuration in YAML
type YAMLPlugin struct {
	ID               string              `yaml:"id"`
	Language         string              `yaml:"language"`
	Title            string              `yaml:"title"`
	Icon             string              `yaml:"icon"`
	Description      string              `yaml:"description"`
	Type             string              `yaml:"type"`
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

// LoadHashiCorpPluginRegistryFromYAML loads plugin configurations from plugins.yml
func LoadHashiCorpPluginRegistryFromYAML(cfg *models.Config) (map[string]*protobuff.PluginDetails, error) {
	// Get the plugins.yml file path
	pluginsYAMLPath := filepath.Join(cfg.PluginPath, "plugins.yml")

	// Check if file exists
	if _, err := os.Stat(pluginsYAMLPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("plugins.yml not found at %s", pluginsYAMLPath)
	}

	// Read the YAML file
	data, err := os.ReadFile(pluginsYAMLPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read plugins.yml: %w", err)
	}

	// Parse YAML
	var config YAMLPluginConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse plugins.yml: %w", err)
	}

	// Convert YAML config to protobuf PluginDetails
	plugins := make(map[string]*protobuff.PluginDetails)

	for pluginID, yamlPlugin := range config.Plugins {
		pluginDetails, err := convertYAMLPluginToProtobuf(yamlPlugin)
		if err != nil {
			return nil, fmt.Errorf("failed to convert plugin %s: %w", pluginID, err)
		}
		plugins[pluginID] = pluginDetails
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

	// Convert type string to enum
	pluginType, err := convertTypeStringToEnum(yamlPlugin.Type)
	if err != nil {
		return nil, err
	}

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
		Type:             pluginType,
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

// convertTypeStringToEnum converts type string to protobuff enum
func convertTypeStringToEnum(pluginType string) (protobuff.PluginType, error) {
	switch strings.ToLower(pluginType) {
	case "system":
		return protobuff.PluginType_PLUGIN_TYPE_SYSTEM, nil
	case "project":
		return protobuff.PluginType_PLUGIN_TYPE_PROJECT, nil
	default:
		return protobuff.PluginType_PLUGIN_TYPE_SYSTEM, fmt.Errorf("unsupported plugin type: %s", pluginType)
	}
}

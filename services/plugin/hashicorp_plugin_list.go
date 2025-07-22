package plugin

import (
	"github.com/apito-io/engine/models"
	"github.com/apito-io/types/protobuff"
)

func LoadHashiCorpPluginRegistry(cfg *models.Config) (map[string]*protobuff.PluginDetails, error) {
	// Use the new YAML-based plugin loader
	return LoadHashiCorpPluginRegistryFromYAML(cfg)
}

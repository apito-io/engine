package database

import (
	"fmt"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

func ResolveDriverFactory(cfg *models.Config) (interfaces.DatabaseDriverFactory, error) {
	if cfg == nil || cfg.DriverFactory == nil {
		return nil, fmt.Errorf("DriverFactory is required; inject open_driver or pro_driver factory at boot")
	}
	f, ok := cfg.DriverFactory.(interfaces.DatabaseDriverFactory)
	if !ok {
		return nil, fmt.Errorf("DriverFactory must implement interfaces.DatabaseDriverFactory")
	}
	return f, nil
}

func ResolveRealtimeBusFactory(cfg *models.Config) (interfaces.RealtimeBusFactory, error) {
	if cfg == nil || cfg.RealtimeBusFactory == nil {
		return nil, fmt.Errorf("RealtimeBusFactory is required; inject nats_system or open_driver factory at boot")
	}
	f, ok := cfg.RealtimeBusFactory.(interfaces.RealtimeBusFactory)
	if !ok {
		return nil, fmt.Errorf("RealtimeBusFactory must implement interfaces.RealtimeBusFactory")
	}
	return f, nil
}

func ResolveCacheFactory(cfg *models.Config) (interfaces.CacheFactory, error) {
	if cfg == nil || cfg.CacheFactory == nil {
		return nil, fmt.Errorf("CacheFactory is required; inject open_driver factory at boot")
	}
	f, ok := cfg.CacheFactory.(interfaces.CacheFactory)
	if !ok {
		return nil, fmt.Errorf("CacheFactory must implement interfaces.CacheFactory")
	}
	return f, nil
}

func ResolveKVFactory(cfg *models.Config) (interfaces.KVFactory, error) {
	if cfg == nil || cfg.KVFactory == nil {
		return nil, fmt.Errorf("KVFactory is required; inject open_driver factory at boot")
	}
	f, ok := cfg.KVFactory.(interfaces.KVFactory)
	if !ok {
		return nil, fmt.Errorf("KVFactory must implement interfaces.KVFactory")
	}
	return f, nil
}

func ResolvePluginHost(cfg *models.Config) (interfaces.PluginHost, error) {
	if cfg == nil || cfg.PluginHost == nil {
		return nil, fmt.Errorf("PluginHost is required; inject plugin_system factory at boot")
	}
	h, ok := cfg.PluginHost.(interfaces.PluginHost)
	if !ok {
		return nil, fmt.Errorf("PluginHost must implement interfaces.PluginHost")
	}
	return h, nil
}

package resolver

import "github.com/apito-io/engine/models"

func (s *GraphQLServer) hashiCorpPlugin(pluginID string) *models.HashiCorpPluginCache {
	if s == nil || pluginID == "" {
		return nil
	}
	s.pluginCacheMu.RLock()
	defer s.pluginCacheMu.RUnlock()
	if s.HashiCorpPluginCache == nil {
		return nil
	}
	return s.HashiCorpPluginCache[pluginID]
}

func (s *GraphQLServer) storeHashiCorpPlugin(pluginID string, cache *models.HashiCorpPluginCache) {
	if s == nil || pluginID == "" {
		return
	}
	s.pluginCacheMu.Lock()
	defer s.pluginCacheMu.Unlock()
	if s.HashiCorpPluginCache == nil {
		s.HashiCorpPluginCache = make(map[string]*models.HashiCorpPluginCache)
	}
	s.HashiCorpPluginCache[pluginID] = cache
}

// RemoveHashiCorpPlugin removes a plugin from the cache under pluginCacheMu.
func (s *GraphQLServer) RemoveHashiCorpPlugin(pluginID string) {
	s.removeHashiCorpPlugin(pluginID)
}

func (s *GraphQLServer) removeHashiCorpPlugin(pluginID string) {
	if s == nil || pluginID == "" {
		return
	}
	s.pluginCacheMu.Lock()
	defer s.pluginCacheMu.Unlock()
	delete(s.HashiCorpPluginCache, pluginID)
}

func (s *GraphQLServer) snapshotHashiCorpPlugins() map[string]*models.HashiCorpPluginCache {
	if s == nil {
		return nil
	}
	s.pluginCacheMu.RLock()
	defer s.pluginCacheMu.RUnlock()
	out := make(map[string]*models.HashiCorpPluginCache, len(s.HashiCorpPluginCache))
	for k, v := range s.HashiCorpPluginCache {
		out[k] = v
	}
	return out
}

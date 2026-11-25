package resolver

import (
	"fmt"
	"maps"
	"slices"

	"github.com/tailor-platform/graphql"
)

// System GraphQL fields
//
// SystemQueries/SystemMutations are read on every system GraphQL request (the
// schema is built per request) and written by the boot-time schema channel and
// by plugin registration, which also runs on hot reload and monitor restart.
// They are copy-on-write: a writer clones the current map, edits the clone and
// publishes it under systemFieldsMu. A published map is never mutated, so a
// reader only needs the lock long enough to copy the two pointers.

// SystemFieldsSnapshot returns the current system query and mutation fields.
// Callers must treat the returned maps as read-only.
func (s *GraphQLServer) SystemFieldsSnapshot() (queries, mutations graphql.Fields) {
	s.systemFieldsMu.RLock()
	defer s.systemFieldsMu.RUnlock()
	return s.SystemQueries, s.SystemMutations
}

// overwriteSystemFields merges core system fields, replacing existing keys.
func (s *GraphQLServer) overwriteSystemFields(queries, mutations graphql.Fields) {
	if len(queries) == 0 && len(mutations) == 0 {
		return
	}
	s.systemFieldsMu.Lock()
	defer s.systemFieldsMu.Unlock()

	if len(queries) > 0 {
		next := cloneFields(s.SystemQueries, len(queries))
		maps.Copy(next, queries)
		s.SystemQueries = next
	}
	if len(mutations) > 0 {
		next := cloneFields(s.SystemMutations, len(mutations))
		maps.Copy(next, mutations)
		s.SystemMutations = next
	}
}

// mergePluginSystemFields adds a plugin's system fields. A key that is already
// present is left untouched unless this plugin registered it before (hot
// reload / restart), in which case the fresh field definition replaces it.
func (s *GraphQLServer) mergePluginSystemFields(pluginID string, queries, mutations graphql.Fields) {
	if len(queries) == 0 && len(mutations) == 0 {
		return
	}
	s.systemFieldsMu.Lock()
	defer s.systemFieldsMu.Unlock()

	if s.systemFieldOwners == nil {
		s.systemFieldOwners = make(map[string]string)
	}

	if len(queries) > 0 {
		s.SystemQueries = s.mergeOwnedFieldsLocked(pluginID, "query", s.SystemQueries, queries)
	}
	if len(mutations) > 0 {
		s.SystemMutations = s.mergeOwnedFieldsLocked(pluginID, "mutation", s.SystemMutations, mutations)
	}
}

// mergeOwnedFieldsLocked returns a new map with incoming merged into current
// following first-owner-wins. systemFieldsMu must be held for writing.
func (s *GraphQLServer) mergeOwnedFieldsLocked(pluginID, kind string, current, incoming graphql.Fields) graphql.Fields {
	next := cloneFields(current, len(incoming))
	for key, field := range incoming {
		ownerKey := kind + ":" + key
		if existing := next[key]; existing != nil && s.systemFieldOwners[ownerKey] != pluginID {
			fmt.Printf("the %s '%s' on '%s' already found on another plugin. please change the id. ignoring this %s\n", kind, key, pluginID, kind)
			continue
		}
		fmt.Printf("--> Registering plugin schema from %s %s `%s` to system schema\n", pluginID, kind, key)
		next[key] = field
		s.systemFieldOwners[ownerKey] = pluginID
	}
	return next
}

// prefixSystemFieldKeys namespaces plugin field names for the system schema.
func prefixSystemFieldKeys(fields graphql.Fields) graphql.Fields {
	if len(fields) == 0 {
		return nil
	}
	out := make(graphql.Fields, len(fields))
	for k, v := range fields {
		out[DEFAULT_INTERNAL_PREFIX+"_"+k] = v
	}
	return out
}

func cloneFields(current graphql.Fields, extra int) graphql.Fields {
	next := make(graphql.Fields, len(current)+extra)
	maps.Copy(next, current)
	return next
}

// Installed HashiCorp plugin IDs

func (s *GraphQLServer) addInstalledHCPlugin(pluginID string) {
	s.pluginRegistryMu.Lock()
	defer s.pluginRegistryMu.Unlock()
	if !slices.Contains(s.InstalledHCPluginList, pluginID) {
		s.InstalledHCPluginList = append(s.InstalledHCPluginList, pluginID)
	}
}

// RemoveInstalledHCPlugin drops pluginID from the installed HashiCorp plugin list.
func (s *GraphQLServer) RemoveInstalledHCPlugin(pluginID string) {
	s.pluginRegistryMu.Lock()
	defer s.pluginRegistryMu.Unlock()
	s.InstalledHCPluginList = slices.DeleteFunc(slices.Clone(s.InstalledHCPluginList), func(id string) bool {
		return id == pluginID
	})
}

// installedHCPluginIDs returns a copy of the installed HashiCorp plugin IDs,
// or nil when none are installed.
func (s *GraphQLServer) installedHCPluginIDs() []string {
	s.pluginRegistryMu.RLock()
	defer s.pluginRegistryMu.RUnlock()
	if len(s.InstalledHCPluginList) == 0 {
		return nil
	}
	return slices.Clone(s.InstalledHCPluginList)
}

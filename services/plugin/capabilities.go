package plugin

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/apito-io/types/protobuff"
)

// Capability identifiers declared by a system-installed plugin.
const (
	CapSystemGraphQL   = "system.graphql"
	CapSystemREST      = "system.rest"
	CapProjectGraphQL  = "project.graphql"
	CapProjectREST     = "project.rest"
	CapConsoleRoutes   = "console.routes"
	CapConsoleSettings = "console.settings"
	CapContentFields   = "content.fields"
	CapEventSink       = "system.events"
)

var knownCapabilities = map[string]struct{}{
	CapSystemGraphQL:   {},
	CapSystemREST:      {},
	CapProjectGraphQL:  {},
	CapProjectREST:     {},
	CapConsoleRoutes:   {},
	CapConsoleSettings: {},
	CapContentFields:   {},
	CapEventSink:       {},
}

// IsKnownCapability reports whether cap is in the allowlist.
func IsKnownCapability(cap string) bool {
	_, ok := knownCapabilities[strings.TrimSpace(strings.ToLower(cap))]
	return ok
}

// KnownCapabilityList returns the sorted allowlist.
func KnownCapabilityList() []string {
	out := make([]string, 0, len(knownCapabilities))
	for c := range knownCapabilities {
		out = append(out, c)
	}
	return out
}

// UIManifest is signed-official Console UI metadata from config.yml.
type UIManifest struct {
	EntryPath    string
	Official     bool
	BundleURL    string
	BundleSHA256 string
	Publisher    string
	Signed       bool
}

var (
	capMu    sync.RWMutex
	capIndex = map[string][]string{}
	uiIndex  = map[string]UIManifest{}
)

// SetPluginCapabilities records declared capabilities for a plugin id.
func SetPluginCapabilities(id string, caps []string) {
	normalized := NormalizeCapabilities(caps)
	capMu.Lock()
	defer capMu.Unlock()
	if len(normalized) == 0 {
		delete(capIndex, id)
		return
	}
	copied := make([]string, len(normalized))
	copy(copied, normalized)
	capIndex[id] = copied
}

// CapabilitiesFor returns declared capabilities for a plugin id.
func CapabilitiesFor(id string) []string {
	capMu.RLock()
	defer capMu.RUnlock()
	src := capIndex[id]
	out := make([]string, len(src))
	copy(out, src)
	return out
}

// HasCapability reports whether plugin id declares cap.
func HasCapability(id, cap string) bool {
	for _, c := range CapabilitiesFor(id) {
		if c == cap {
			return true
		}
	}
	return false
}

// PluginIDFromSource extracts a plugin id from GraphQL/REST source values.
func PluginIDFromSource(source interface{}) string {
	switch v := source.(type) {
	case *protobuff.PluginDetails:
		if v != nil {
			return v.Id
		}
	case protobuff.PluginDetails:
		return v.Id
	case map[string]interface{}:
		if id, ok := v["id"].(string); ok {
			return id
		}
		if id, ok := v["ID"].(string); ok {
			return id
		}
	}
	if v, ok := source.(interface{ GetId() string }); ok && v != nil {
		return v.GetId()
	}
	return ""
}

// SetPluginUIManifest records official UI metadata for a plugin id.
func SetPluginUIManifest(id string, ui UIManifest) {
	capMu.Lock()
	defer capMu.Unlock()
	if ui == (UIManifest{}) {
		delete(uiIndex, id)
		return
	}
	uiIndex[id] = ui
}

// UIManifestFor returns official UI metadata for a plugin id.
func UIManifestFor(id string) UIManifest {
	capMu.RLock()
	defer capMu.RUnlock()
	return uiIndex[id]
}

// NormalizeCapabilities drops unknown values and de-duplicates.
func NormalizeCapabilities(caps []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range caps {
		c := strings.TrimSpace(strings.ToLower(raw))
		if c == "" {
			continue
		}
		if _, ok := knownCapabilities[c]; !ok {
			continue
		}
		if _, dup := seen[c]; dup {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

// CapabilitiesFromLegacyType maps retired YAML `type:` to capabilities.
// `project` is accepted only as a migration shim.
func CapabilitiesFromLegacyType(pluginType string) ([]string, error) {
	switch strings.ToLower(strings.TrimSpace(pluginType)) {
	case "", "system", "internal":
		return []string{CapSystemGraphQL, CapSystemREST}, nil
	case "project", "external":
		return []string{CapProjectGraphQL, CapProjectREST}, nil
	default:
		return nil, fmt.Errorf("unsupported plugin type %q; use capabilities instead of type", pluginType)
	}
}

// RejectLegacyProjectTypeInDev returns a migration error in development when
// config.yml still uses type: project without explicit capabilities.
func RejectLegacyProjectTypeInDev(pluginType string, caps []string) error {
	if len(NormalizeCapabilities(caps)) > 0 {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(pluginType)) {
	case "project", "external":
		if isDevEnv() {
			return fmt.Errorf("type: project is removed; declare capabilities: [%s, %s]", CapProjectGraphQL, CapProjectREST)
		}
	}
	return nil
}

func isDevEnv() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENVIRONMENT")))
	if env == "" {
		env = strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	}
	switch env {
	case "dev", "development", "local", "test":
		return true
	}
	return os.Getenv("PLUGIN_STRICT_TYPE") == "1"
}

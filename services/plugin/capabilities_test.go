package plugin

import (
	"os"
	"testing"
)

func TestNormalizeCapabilities(t *testing.T) {
	got := NormalizeCapabilities([]string{
		" project.graphql ",
		"PROJECT.GRAPHQL",
		"nope",
		"system.rest",
	})
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
	if got[0] != CapProjectGraphQL || got[1] != CapSystemREST {
		t.Fatalf("got %#v", got)
	}
}

func TestCapabilitiesFromLegacyType(t *testing.T) {
	caps, err := CapabilitiesFromLegacyType("project")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(caps, CapProjectGraphQL) || !contains(caps, CapProjectREST) {
		t.Fatalf("project legacy %#v", caps)
	}
	caps, err = CapabilitiesFromLegacyType("system")
	if err != nil {
		t.Fatal(err)
	}
	if !contains(caps, CapSystemGraphQL) {
		t.Fatalf("system legacy %#v", caps)
	}
}

func TestRejectLegacyProjectTypeInDev(t *testing.T) {
	t.Setenv("PLUGIN_STRICT_TYPE", "1")
	if err := RejectLegacyProjectTypeInDev("project", nil); err == nil {
		t.Fatal("expected migration error")
	}
	if err := RejectLegacyProjectTypeInDev("project", []string{CapProjectGraphQL}); err != nil {
		t.Fatal(err)
	}
	_ = os.Unsetenv("PLUGIN_STRICT_TYPE")
}

func TestCapabilityIndex(t *testing.T) {
	SetPluginCapabilities("hc-foo-plugin", []string{CapProjectGraphQL, "bogus"})
	t.Cleanup(func() { SetPluginCapabilities("hc-foo-plugin", nil) })
	if !HasCapability("hc-foo-plugin", CapProjectGraphQL) {
		t.Fatal("missing project.graphql")
	}
	if HasCapability("hc-foo-plugin", CapSystemREST) {
		t.Fatal("unexpected system.rest")
	}
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

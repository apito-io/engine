package registry

import "testing"

func TestMergeLocalAndRemote(t *testing.T) {
	cat := &Catalog{SchemaVersion: 1, Plugins: []CatalogEntry{{
		ID: "hc-test-plugin", Name: "Test", PluginVersion: "1.1.0",
		Status: "available", EngineSemver: ">=2.0.0",
		Releases: []ReleaseAsset{{OS: "linux", Arch: "amd64", URL: "https://github.com/apito-io/x/releases/download/v1/x.zip", SHA256: "ab"}},
	}}}
	out := Merge(cat, map[string]InstalledInfo{"hc-test-plugin": {Version: "1.0.0", Health: "loaded"}}, "2.4.49", "linux", "amd64")
	if len(out) != 1 {
		t.Fatalf("len=%d", len(out))
	}
	if !out[0].Installed || !out[0].UpdateAvailable || out[0].Health != "loaded" {
		t.Fatalf("%+v", out[0])
	}
}

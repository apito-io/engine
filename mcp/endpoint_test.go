package mcp

import "testing"

func TestCanonicalResourceStripsClientHints(t *testing.T) {
	got, err := CanonicalResource("https://mcp.apito.io/mcp?kind=local&profile=enterprise")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://mcp.apito.io/mcp" {
		t.Fatalf("resource = %s", got)
	}
}

func TestKindFollowsHost(t *testing.T) {
	ep, err := Resolve("https://mcp.apito.io/mcp?kind=local", "local")
	if err != nil {
		t.Fatal(err)
	}
	if ep.Kind != KindCloud {
		t.Fatalf("kind = %s, host must win", ep.Kind)
	}
	local, err := Resolve("http://localhost:4055/mcp", "")
	if err != nil {
		t.Fatal(err)
	}
	if local.Kind != KindLocal {
		t.Fatalf("local kind = %s", local.Kind)
	}
	custom, err := Resolve("https://enterprise.example.com/mcp", "")
	if err != nil {
		t.Fatal(err)
	}
	if custom.Kind != KindCustom {
		t.Fatalf("custom kind = %s", custom.Kind)
	}
}

func TestDefaultURLsStayClean(t *testing.T) {
	cloud, err := DefaultURL("cloud", "")
	if err != nil || cloud != CloudMCPURL {
		t.Fatalf("cloud = %s %v", cloud, err)
	}
	local, err := DefaultURL("local", "")
	if err != nil || local != LocalMCPURL {
		t.Fatalf("local = %s %v", local, err)
	}
	custom, err := DefaultURL("enterprise", "https://enterprise.example.com/mcp?profile=a")
	if err != nil || custom != "https://enterprise.example.com/mcp" {
		t.Fatalf("custom = %s %v", custom, err)
	}
}

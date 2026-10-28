package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStudioNginxRoutesProtocolToEngine(t *testing.T) {
	path := filepath.Join("..", "..", "..", "studio-oss", "nginx.conf")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{
		"location /mcp",
		"location @mcp_engine",
		"proxy_pass http://127.0.0.1:5055",
		"X-Forwarded-Host",
		"X-Forwarded-Port",
		"proxy_buffering off",
		"Mcp-Session-Id",
		"/.well-known/oauth-protected-resource/mcp",
		"/.well-known/oauth-authorization-server",
		"location /oauth/",
		`$http_mcp_protocol_version`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("nginx.conf missing %s", want)
		}
	}
}

package registry

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestGetFallsBackToDiskWhenLiveFails(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"schema_version":1,"generated_at":"2026-01-01T00:00:00Z","plugins":[{"id":"hc-a-plugin","name":"A","plugin_version":"1.0.0","status":"available"}]}`)
	sig := []byte(hex.EncodeToString(SignCatalog(body, priv)))
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "catalog.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "catalog.sig"), append(sig, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &Client{
		URL:       "https://github.com/apito-io/this-does-not-exist/releases/download/x/catalog.json",
		PublicKey: pub,
		CacheDir:  tmp,
	}
	cat, _, err := c.Get(context.Background())
	if err != nil {
		t.Fatalf("disk fallback: %v", err)
	}
	if len(cat.Plugins) != 1 || cat.Plugins[0].ID != "hc-a-plugin" {
		t.Fatalf("got %+v", cat.Plugins)
	}
}

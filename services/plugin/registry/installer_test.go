package registry

import (
	"archive/zip"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompatibleEngine(t *testing.T) {
	if !CompatibleEngine("2.4.49", ">=2.4.0") {
		t.Fatal("expected compatible")
	}
	if CompatibleEngine("2.3.0", ">=2.4.0") {
		t.Fatal("expected incompatible")
	}
}

func TestVerifyCatalogSignature(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"schema_version":1,"plugins":[]}`)
	sig := SignCatalog(body, priv)
	if err := VerifyCatalogSignature(body, sig, pub); err != nil {
		t.Fatal(err)
	}
	if err := VerifyCatalogSignature(body, []byte("nope"), pub); err == nil {
		t.Fatal("expected invalid signature")
	}
	hexSig := []byte(hex.EncodeToString(sig))
	if err := VerifyCatalogSignature(body, hexSig, pub); err != nil {
		t.Fatal(err)
	}
}

func TestSafeExtractZipRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bad.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	fw, err := w.Create("../evil")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write([]byte("x"))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	dest := filepath.Join(dir, "out")
	err = SafeExtractZip(zipPath, dest, "bin", 0)
	if err == nil {
		t.Fatal("expected zip-slip rejection")
	}
	if !strings.Contains(err.Error(), CodeUnsafeArchive) {
		t.Fatalf("got %v", err)
	}
}

func writeGoodZip(t *testing.T, dir, binaryName string) (zipPath string, size int64, sha string) {
	t.Helper()
	zipPath = filepath.Join(dir, "plugin.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	cfg := `plugin:
  id: "hc-test-plugin"
  version: "1.0.0"
  binary_path: "hc-test-plugin"
  capabilities:
    - project.graphql
  handshake_config:
    protocol_version: 1
    magic_cookie_key: "APITO_PLUGIN"
    magic_cookie_value: "apito_plugin_magic_cookie_v1"
`
	cw, err := zw.Create("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = cw.Write([]byte(cfg))
	bw, err := zw.Create(binaryName)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = bw.Write([]byte("#!/bin/sh\necho test\n"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	b, err := os.ReadFile(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	return zipPath, int64(len(b)), DigestSHA256(b)
}

func TestInstallLifecycle(t *testing.T) {
	AllowLocalhostDownloads = true
	t.Cleanup(func() { AllowLocalhostDownloads = false })

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	_, size, sha := writeGoodZip(t, tmp, "hc-test-plugin")
	zipBytes, err := os.ReadFile(filepath.Join(tmp, "plugin.zip"))
	if err != nil {
		t.Fatal(err)
	}

	var catalogJSON []byte
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	entry := CatalogEntry{
		ID:            "hc-test-plugin",
		Name:          "Test",
		Publisher:     "Apito",
		Repository:    "https://github.com/apito-io/hc-test-plugin",
		License:       "Apache-2.0",
		Description:   "test",
		Category:      "data",
		Language:      "go",
		Capabilities:  []string{"project.graphql"},
		EngineSemver:  ">=2.0.0",
		PluginVersion: "1.0.0",
		Trust:         "official",
		Status:        "available",
		Runtime: RuntimeContract{
			ID: "hc-test-plugin", Version: "1.0.0", BinaryPath: "hc-test-plugin",
			Capabilities: []string{"project.graphql"},
			Handshake:    Handshake{ProtocolVersion: 1, MagicCookieKey: "APITO_PLUGIN", MagicCookieValue: "apito_plugin_magic_cookie_v1"},
		},
		Releases: []ReleaseAsset{{
			OS: "darwin", Arch: "arm64",
			URL: srv.URL + "/plugin.zip", Size: size, SHA256: sha,
		}},
	}
	cat := Catalog{SchemaVersion: 1, GeneratedAt: "2026-09-03T00:00:00Z", Plugins: []CatalogEntry{entry}}
	catalogJSON, err = json.Marshal(cat)
	if err != nil {
		t.Fatal(err)
	}
	sig := SignCatalog(catalogJSON, priv)

	mux.HandleFunc("/catalog.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write(catalogJSON)
	})
	mux.HandleFunc("/catalog.sig", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(sig)
	})
	mux.HandleFunc("/plugin.zip", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipBytes)
	})

	pluginPath := filepath.Join(tmp, "plugins")
	client := &Client{
		URL: srv.URL + "/catalog.json", SigURL: srv.URL + "/catalog.sig",
		PublicKey: pub, CacheDir: filepath.Join(tmp, "cache"),
	}
	loaded := false
	in := &Installer{
		PluginPath: pluginPath, EngineVersion: "2.4.49", Client: client,
		GOOS: "darwin", GOARCH: "arm64",
		HotLoad: func(id string) error { loaded = true; return nil },
		Stop:    func(id string) error { return nil },
	}
	rec, err := in.Install(context.Background(), "hc-test-plugin", "1.0.0", "tester")
	if err != nil {
		t.Fatal(err)
	}
	if rec.Version != "1.0.0" || !loaded {
		t.Fatalf("record=%+v loaded=%v", rec, loaded)
	}
	if _, err := os.Stat(filepath.Join(pluginPath, "hc-test-plugin", "config.yml")); err != nil {
		t.Fatal(err)
	}

	in.ActiveProjects = func(ctx context.Context, pluginID string) ([]string, error) {
		return []string{"proj-1"}, nil
	}
	if err := in.Uninstall(context.Background(), "hc-test-plugin", false); err == nil {
		t.Fatal("expected in-use")
	}
	if err := in.Uninstall(context.Background(), "hc-test-plugin", true); err != nil {
		t.Fatal(err)
	}
}

func TestChecksumMismatch(t *testing.T) {
	AllowLocalhostDownloads = true
	t.Cleanup(func() { AllowLocalhostDownloads = false })
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	_, size, _ := writeGoodZip(t, tmp, "hc-test-plugin")
	zipBytes, _ := os.ReadFile(filepath.Join(tmp, "plugin.zip"))

	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	entry := CatalogEntry{
		ID: "hc-test-plugin", Name: "Test", Publisher: "Apito",
		Repository: "https://github.com/apito-io/hc-test-plugin", License: "Apache-2.0",
		Description: "t", Category: "data", Language: "go",
		Capabilities: []string{"project.graphql"}, EngineSemver: ">=2.0.0",
		PluginVersion: "1.0.0", Trust: "official", Status: "available",
		Runtime: RuntimeContract{
			ID: "hc-test-plugin", Version: "1.0.0", BinaryPath: "hc-test-plugin",
			Capabilities: []string{"project.graphql"},
			Handshake:    Handshake{1, "APITO_PLUGIN", "apito_plugin_magic_cookie_v1"},
		},
		Releases: []ReleaseAsset{{OS: "linux", Arch: "amd64", URL: srv.URL + "/plugin.zip", Size: size, SHA256: strings.Repeat("ab", 32)}},
	}
	body, _ := json.Marshal(Catalog{SchemaVersion: 1, Plugins: []CatalogEntry{entry}})
	sig := SignCatalog(body, priv)
	mux.HandleFunc("/catalog.json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) })
	mux.HandleFunc("/catalog.sig", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(sig) })
	mux.HandleFunc("/plugin.zip", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(zipBytes) })

	in := &Installer{
		PluginPath: filepath.Join(tmp, "plugins"), EngineVersion: "2.4.49",
		Client: &Client{URL: srv.URL + "/catalog.json", SigURL: srv.URL + "/catalog.sig", PublicKey: pub, CacheDir: filepath.Join(tmp, "c")},
		GOOS:   "linux", GOARCH: "amd64",
	}
	_, err = in.Install(context.Background(), "hc-test-plugin", "", "t")
	if err == nil || !strings.Contains(err.Error(), CodeChecksumMismatch) {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestRejectCatalogStub(t *testing.T) {
	AllowLocalhostDownloads = true
	t.Cleanup(func() { AllowLocalhostDownloads = false })
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	entry := CatalogEntry{
		ID: "hc-stub-plugin", Name: "Stub", Publisher: "Apito",
		Repository: "https://github.com/apito-io/hc-stub-plugin", License: "Apache-2.0",
		Description: "s", Category: "data", Language: "go", PluginVersion: "1.0.0",
		Trust: "official", Status: "catalog-stub", Runtime: RuntimeContract{ID: "hc-stub-plugin", Version: "1.0.0"},
	}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	body, _ := json.Marshal(Catalog{SchemaVersion: 1, Plugins: []CatalogEntry{entry}})
	sig := SignCatalog(body, priv)
	mux.HandleFunc("/catalog.json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) })
	mux.HandleFunc("/catalog.sig", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(sig) })
	in := &Installer{
		PluginPath: filepath.Join(tmp, "p"), EngineVersion: "2.4.49",
		Client: &Client{URL: srv.URL + "/catalog.json", SigURL: srv.URL + "/catalog.sig", PublicKey: pub, CacheDir: filepath.Join(tmp, "c")},
	}
	_, err = in.Install(context.Background(), "hc-stub-plugin", "", "t")
	if err == nil || !strings.Contains(err.Error(), CodeNotInstallable) {
		t.Fatalf("expected not installable, got %v", err)
	}
}

func TestSafeExtractZipKeepsUiJS(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "plugin.zip")
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	cw, err := zw.Create("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = cw.Write([]byte("plugin:\n  id: hc-test-plugin\n"))
	bw, err := zw.Create("hc-test-plugin")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = bw.Write([]byte("bin"))
	uw, err := zw.Create("ui.js")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = uw.Write([]byte("window.ApitoPluginSDK.register({});"))
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()
	dest := filepath.Join(dir, "out")
	if err := SafeExtractZip(zipPath, dest, "hc-test-plugin", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "ui.js")); err != nil {
		t.Fatalf("ui.js missing: %v", err)
	}
}

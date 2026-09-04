package registry

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Client fetches and caches the signed catalog.
type Client struct {
	URL        string
	SigURL     string
	PublicKey  ed25519.PublicKey
	HTTP       *http.Client
	CacheDir   string
	StaleAfter time.Duration

	mu        sync.Mutex
	cached    *Catalog
	raw       []byte
	digest    string
	etag      string
	fetchedAt time.Time
}

func (c *Client) sigURL() string {
	if strings.TrimSpace(c.SigURL) != "" {
		return c.SigURL
	}
	u := c.URL
	if strings.HasSuffix(u, ".json") {
		return strings.TrimSuffix(u, ".json") + ".sig"
	}
	return u + ".sig"
}

func (c *Client) http() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second, CheckRedirect: boundedRedirects}
}

func (c *Client) fetchLive(ctx context.Context) (*Catalog, string, error) {
	raw, etag, err := c.fetchBytes(ctx, c.URL, c.etag)
	if err != nil {
		return nil, "", err
	}
	if raw == nil && c.cached != nil {
		return c.cached, c.digest, nil
	}
	if len(raw) == 0 {
		return nil, "", coded(CodeRegistryUnavailable, "empty catalog body", nil)
	}

	sig, _, err := c.fetchBytes(ctx, c.sigURL(), "")
	if err != nil || len(sig) == 0 {
		return nil, "", coded(CodeSignatureInvalid, "cannot fetch catalog signature", err)
	}
	if err := VerifyCatalogSignature(raw, sig, c.PublicKey); err != nil {
		return nil, "", err
	}
	cat, err := ParseCatalogJSON(raw)
	if err != nil {
		return nil, "", err
	}
	digest := DigestSHA256(raw)
	c.cached = cat
	c.raw = raw
	c.digest = digest
	c.etag = etag
	c.fetchedAt = time.Now()
	_ = c.saveDisk(raw, sig, digest)
	return cat, digest, nil
}

// Get returns the verified catalog, using ETag cache and bounded stale fallback.
func (c *Client) Get(ctx context.Context) (*Catalog, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cat, digest, err := c.fetchLive(ctx)
	if err == nil {
		return cat, digest, nil
	}
	if c.cached != nil && time.Since(c.fetchedAt) < c.staleWindow() {
		return c.cached, c.digest, nil
	}
	if disk, dgst, diskErr := c.loadDisk(); diskErr == nil {
		return disk, dgst, nil
	}
	return nil, "", err
}

func (c *Client) staleWindow() time.Duration {
	if c.StaleAfter > 0 {
		return c.StaleAfter
	}
	return 6 * time.Hour
}

func (c *Client) fetchBytes(ctx context.Context, rawURL, etag string) ([]byte, string, error) {
	if strings.HasPrefix(rawURL, "file://") {
		b, err := os.ReadFile(strings.TrimPrefix(rawURL, "file://"))
		return b, "", err
	}
	if err := rejectBadURL(rawURL); err != nil {
		// file:// already handled; tests may use http localhost
		if !AllowLocalhostDownloads {
			return nil, "", err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	req.Header.Set("User-Agent", "ApitoEngine-plugin-catalog")
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := c.http().Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return nil, etag, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, "", err
	}
	return body, resp.Header.Get("ETag"), nil
}

func (c *Client) diskPaths() (jsonPath, sigPath string) {
	dir := c.CacheDir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "apito-plugin-catalog")
	}
	return filepath.Join(dir, "catalog.json"), filepath.Join(dir, "catalog.sig")
}

func (c *Client) saveDisk(raw, sig []byte, digest string) error {
	jsonPath, sigPath := c.diskPaths()
	if err := os.MkdirAll(filepath.Dir(jsonPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, raw, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(sigPath, sig, 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(filepath.Dir(jsonPath), "digest"), []byte(digest), 0o644)
}

func (c *Client) loadDisk() (*Catalog, string, error) {
	jsonPath, sigPath := c.diskPaths()
	raw, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, "", err
	}
	sig, err := os.ReadFile(sigPath)
	if err != nil {
		return nil, "", err
	}
	if err := VerifyCatalogSignature(raw, sig, c.PublicKey); err != nil {
		return nil, "", err
	}
	cat, err := ParseCatalogJSON(raw)
	if err != nil {
		return nil, "", err
	}
	return cat, DigestSHA256(raw), nil
}

// Find returns a plugin by id.
func (cat *Catalog) Find(id string) *CatalogEntry {
	for i := range cat.Plugins {
		if cat.Plugins[i].ID == id {
			return &cat.Plugins[i]
		}
	}
	return nil
}

// SelectRelease picks GOOS/GOARCH from the entry.
func (e *CatalogEntry) SelectRelease(goos, goarch string) *ReleaseAsset {
	for i := range e.Releases {
		r := &e.Releases[i]
		if r.OS == goos && r.Arch == goarch {
			return r
		}
	}
	return nil
}

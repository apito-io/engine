package registry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	defaultMaxDownload = 128 << 20 // 128 MiB
	defaultTimeout     = 60 * time.Second
	maxRedirects       = 3
)

var allowedHosts = map[string]struct{}{
	"github.com":                            {},
	"objects.githubusercontent.com":         {},
	"release-assets.githubusercontent.com":  {},
	"github-releases.githubusercontent.com": {},
}

// AllowLocalhostDownloads is for tests (httptest.Server).
var AllowLocalhostDownloads bool

func hostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if _, ok := allowedHosts[host]; ok {
		return true
	}
	if AllowLocalhostDownloads && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
		return true
	}
	return false
}

func rejectBadURL(raw string) error {
	if strings.Contains(strings.ToLower(raw), "/latest") {
		return coded(CodeHostNotAllowed, "mutable latest URLs are not allowed", nil)
	}
	u, err := url.Parse(raw)
	if err != nil {
		return coded(CodeHostNotAllowed, "invalid asset URL", err)
	}
	if u.Scheme != "https" && !(AllowLocalhostDownloads && u.Scheme == "http") {
		return coded(CodeHostNotAllowed, "asset URL must be https", nil)
	}
	if !hostAllowed(u.Host) {
		return coded(CodeHostNotAllowed, "asset host is not on the GitHub allowlist: "+u.Host, nil)
	}
	return nil
}

// DownloadVerified fetches url, enforces host/size/sha256, writes destPath.
func DownloadVerified(ctx context.Context, client *http.Client, rawURL string, expectedSize int64, expectedSHA string, destPath string, maxBytes int64) error {
	if err := rejectBadURL(rawURL); err != nil {
		return err
	}
	if maxBytes <= 0 {
		maxBytes = defaultMaxDownload
	}
	if expectedSize > maxBytes {
		return coded(CodeSizeMismatch, "declared size exceeds download limit", nil)
	}
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout, CheckRedirect: boundedRedirects}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return coded(CodeDownloadFailed, "build request", err)
	}
	req.Header.Set("Accept", "application/octet-stream")
	resp, err := client.Do(req)
	if err != nil {
		return coded(CodeDownloadFailed, "download failed", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return coded(CodeDownloadFailed, fmt.Sprintf("download status %d", resp.StatusCode), nil)
	}
	if resp.ContentLength > 0 && expectedSize > 0 && resp.ContentLength != expectedSize {
		return coded(CodeSizeMismatch, fmt.Sprintf("content-length %d != declared %d", resp.ContentLength, expectedSize), nil)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	h := sha256.New()
	n, err := io.Copy(out, io.TeeReader(io.LimitReader(resp.Body, maxBytes+1), h))
	if err != nil {
		return coded(CodeDownloadFailed, "read body", err)
	}
	if n > maxBytes {
		return coded(CodeSizeMismatch, "download exceeded maximum size", nil)
	}
	if expectedSize > 0 && n != expectedSize {
		return coded(CodeSizeMismatch, fmt.Sprintf("downloaded %d bytes, expected %d", n, expectedSize), nil)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, strings.TrimSpace(expectedSHA)) {
		return coded(CodeChecksumMismatch, "SHA-256 does not match catalog", nil)
	}
	return nil
}

func boundedRedirects(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("too many redirects")
	}
	if err := rejectBadURL(req.URL.String()); err != nil {
		return err
	}
	return nil
}

package mcp

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

const (
	KindCloud  = "cloud"
	KindLocal  = "local"
	KindCustom = "custom"

	CloudMCPURL = "https://api.apito.io/mcp"
	LocalMCPURL = "http://localhost:4055/mcp"

	ScopeMCP = "mcp"
)

// Endpoint is the public MCP resource a client configures.
type Endpoint struct {
	Kind     string
	Resource string
	Public   string
}

// CanonicalResource removes client hints from the resource identifier.
// kind and profile are not part of the OAuth audience.
func CanonicalResource(raw string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("mcp url is invalid")
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return "", fmt.Errorf("mcp url must be http or https")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	parsed.User = nil
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	if parsed.Path == "" {
		parsed.Path = "/mcp"
	}
	if !strings.HasSuffix(parsed.Path, "/mcp") {
		return "", fmt.Errorf("mcp url must end in /mcp")
	}
	return parsed.String(), nil
}

// InferKind reads the deployment from the URL. The host wins over any hint.
func InferKind(raw string) (string, error) {
	resource, err := CanonicalResource(raw)
	if err != nil {
		return "", err
	}
	parsed, _ := url.Parse(resource)
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "mcp.apito.io", "api.apito.io":
		return KindCloud, nil
	case "localhost", "127.0.0.1", "::1":
		return KindLocal, nil
	default:
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return KindLocal, nil
		}
		return KindCustom, nil
	}
}

// Resolve applies an optional kind hint without letting it override the host.
func Resolve(raw, kindHint string) (Endpoint, error) {
	resource, err := CanonicalResource(raw)
	if err != nil {
		return Endpoint{}, err
	}
	kind, err := InferKind(resource)
	if err != nil {
		return Endpoint{}, err
	}
	hint := strings.ToLower(strings.TrimSpace(kindHint))
	if hint == "" {
		hint = KindCloud
	}
	switch hint {
	case KindCloud, KindLocal, KindCustom, "enterprise", "self-hosted", "oss":
	default:
		return Endpoint{}, fmt.Errorf("kind must be cloud, local, or custom")
	}
	return Endpoint{Kind: kind, Resource: resource, Public: resource}, nil
}

// DefaultURL is the clean URL for a deployment. It does not add kind or profile.
func DefaultURL(kind, custom string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", KindCloud:
		return CloudMCPURL, nil
	case KindLocal, "oss":
		return LocalMCPURL, nil
	case KindCustom, "enterprise", "self-hosted":
		return CanonicalResource(custom)
	default:
		return "", fmt.Errorf("kind must be cloud, local, or custom")
	}
}

// PublicBase is scheme://host[:port] for the request the client actually used.
func PublicBase(scheme, host, forwardedProto, forwardedHost, configured string) string {
	if configured = strings.TrimRight(strings.TrimSpace(configured), "/"); configured != "" {
		return configured
	}
	if forwardedHost != "" {
		host = forwardedHost
	}
	if forwardedProto != "" {
		scheme = forwardedProto
	}
	if scheme == "" {
		scheme = "http"
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	return scheme + "://" + host
}

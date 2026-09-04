package registry

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type yamlPluginFile struct {
	Plugin struct {
		ID           string   `yaml:"id"`
		Version      string   `yaml:"version"`
		BinaryPath   string   `yaml:"binary_path"`
		Capabilities []string `yaml:"capabilities"`
		Handshake    struct {
			ProtocolVersion  int32  `yaml:"protocol_version"`
			MagicCookieKey   string `yaml:"magic_cookie_key"`
			MagicCookieValue string `yaml:"magic_cookie_value"`
		} `yaml:"handshake_config"`
	} `yaml:"plugin"`
}

// First plugin.version line in config.yml (not protocol_version).
var yamlPluginVersionLine = regexp.MustCompile(`(?m)^(\s*version:\s*)(["']?)([^"'\n]+)(["']?)\s*$`)

func stampYAMLVersion(raw []byte, version string) []byte {
	done := false
	return yamlPluginVersionLine.ReplaceAllFunc(raw, func(m []byte) []byte {
		if done {
			return m
		}
		done = true
		sub := yamlPluginVersionLine.FindSubmatch(m)
		q := string(sub[2])
		if q == "" {
			q = `"`
		}
		return []byte(string(sub[1]) + q + version + q)
	})
}

func matchRuntime(configPath string, want RuntimeContract) error {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return coded(CodeConfigMismatch, "cannot read extracted config.yml", err)
	}
	var y yamlPluginFile
	if err := yaml.Unmarshal(b, &y); err != nil {
		return coded(CodeConfigMismatch, "config.yml is not valid YAML", err)
	}
	p := y.Plugin
	if p.ID != want.ID {
		return coded(CodeConfigMismatch, fmt.Sprintf("config id %q != catalog %q", p.ID, want.ID), nil)
	}
	if p.BinaryPath != want.BinaryPath {
		return coded(CodeConfigMismatch, fmt.Sprintf("config binary_path %q != catalog %q", p.BinaryPath, want.BinaryPath), nil)
	}
	if !capsEqual(p.Capabilities, want.Capabilities) {
		return coded(CodeConfigMismatch, "config capabilities do not match catalog", nil)
	}
	if p.Handshake.ProtocolVersion != want.Handshake.ProtocolVersion ||
		p.Handshake.MagicCookieKey != want.Handshake.MagicCookieKey ||
		p.Handshake.MagicCookieValue != want.Handshake.MagicCookieValue {
		return coded(CodeConfigMismatch, "handshake does not match catalog", nil)
	}
	// Zip authors sometimes forget to bump config.yml when tagging. Catalog version
	// is the approved one — stamp it so installed_version matches the marketplace.
	if p.Version != want.Version {
		if err := os.WriteFile(configPath, stampYAMLVersion(b, want.Version), 0o644); err != nil {
			return coded(CodeConfigMismatch, "cannot stamp catalog version onto config.yml", err)
		}
	}
	return nil
}

func capsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	idx := map[string]int{}
	for _, c := range a {
		idx[strings.ToLower(strings.TrimSpace(c))]++
	}
	for _, c := range b {
		k := strings.ToLower(strings.TrimSpace(c))
		if idx[k] == 0 {
			return false
		}
		idx[k]--
	}
	return true
}

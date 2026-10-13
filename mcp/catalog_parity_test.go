package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCatalogMatchesTypeScriptToolNames(t *testing.T) {
	root := filepath.Join("..", "..", "..", "apito-mcp", "src")
	ignore := map[string]bool{
		"confirm_field_type": true,
		"field_name":         true,
		"suggested_types":    true,
	}
	have := map[string]bool{}
	for _, tool := range Catalog() {
		have[tool.Name] = true
	}
	pattern := regexp.MustCompile(`name:\s*'([a-z0-9_]+)'`)
	found := 0
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".ts") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, match := range pattern.FindAllStringSubmatch(string(raw), -1) {
			name := match[1]
			if ignore[name] {
				continue
			}
			found++
			if !have[name] {
				t.Errorf("TypeScript tool %s is missing from the Go catalog", name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found < 100 {
		t.Fatalf("found %d TypeScript tool names", found)
	}
}

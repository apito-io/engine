package utility

import (
	"strings"
	"testing"
)

func TestSQLiteProjectFileName(t *testing.T) {
	n := SQLiteProjectFileName("my-proj/1")
	if !strings.HasSuffix(strings.ToLower(n), ".sqlite") {
		t.Fatalf("expected .sqlite suffix, got %q", n)
	}
	if strings.Contains(n, "/") {
		t.Fatalf("expected sanitized name, got %q", n)
	}
	if SQLiteProjectFileName("") == "" {
		t.Fatal("empty input should yield non-empty fallback")
	}
}

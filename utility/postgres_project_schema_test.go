package utility

import (
	"strings"
	"testing"
)

func TestPostgresProjectSchemaName(t *testing.T) {
	s := PostgresProjectSchemaName("fitness_abcdef")
	if !strings.HasPrefix(s, "p_") {
		t.Fatalf("expected p_ prefix, got %q", s)
	}
	if strings.ContainsAny(s, " -") {
		t.Fatalf("unexpected chars: %q", s)
	}
	if PostgresProjectSchemaName("") == "" {
		t.Fatal("empty input should yield non-empty fallback")
	}
}

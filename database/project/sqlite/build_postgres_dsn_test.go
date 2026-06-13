package sqlite

import (
	"strings"
	"testing"

	"github.com/apito-io/engine/models"
)

func TestBuildPostgresDSN_WithSchemaSearchPath(t *testing.T) {
	c := &models.DriverCredentials{
		Host:     "127.0.0.1",
		Port:     "5432",
		Database: "apito_shared",
		User:     "u",
		Password: "p",
		Schema:   "p_myproj",
	}
	dsn, err := BuildPostgresDSN(c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(dsn, "options=") {
		t.Fatalf("expected options= in DSN for schema isolation, got %q", dsn)
	}
	if !strings.Contains(dsn, "search_path") {
		t.Fatalf("expected search_path in options, got %q", dsn)
	}
}

package sql

import (
	"strings"
	"testing"
	"time"

	"github.com/apito-io/engine/models"
)

func TestSelectBuilder_NoUserFields(t *testing.T) {
	parts := SelectBuilder("y", "", &models.ModelType{Name: "author", Fields: nil}, false)
	q := strings.Join(parts, ", ")
	if strings.Contains(q, ", ,") {
		t.Fatalf("double comma in select (invalid SQL): %q", q)
	}
	if !strings.Contains(q, "x.id AS id") {
		t.Fatalf("expected id column: %q", q)
	}
	if !strings.Contains(q, "sys_created_at") {
		t.Fatalf("expected meta columns: %q", q)
	}
}

func TestSelectBuilder_WithFields(t *testing.T) {
	parts := SelectBuilder("y", "", &models.ModelType{
		Name: "author",
		Fields: []*models.FieldInfo{
			{Identifier: "name"},
		},
	}, false)
	q := strings.Join(parts, ", ")
	if strings.Contains(q, ", ,") {
		t.Fatalf("double comma: %q", q)
	}
	if !strings.Contains(q, "x.name AS name") {
		t.Fatalf("expected field projection: %q", q)
	}
}

func TestFormatSQLMetaTimestamp_SQLiteDateString(t *testing.T) {
	s, err := formatSQLMetaTimestamp("2026-04-01")
	if err != nil {
		t.Fatal(err)
	}
	if want := "2026-04-01T00:00:00Z"; s != want {
		t.Fatalf("got %q want %q", s, want)
	}
}

func TestFormatSQLMetaTimestamp_timeTime(t *testing.T) {
	tm := time.Date(2026, 4, 1, 12, 30, 0, 0, time.UTC)
	s, err := formatSQLMetaTimestamp(tm)
	if err != nil {
		t.Fatal(err)
	}
	if want := "2026-04-01T12:30:00Z"; s != want {
		t.Fatalf("got %q want %q", s, want)
	}
}

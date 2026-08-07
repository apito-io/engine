package resolver

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/types"
)

func TestExistingDocumentFromLookup_SqliteStyleNotFoundIsRecoverable(t *testing.T) {
	// sqlite: fmt.Errorf("document %s not found", id)
	_, found, err := existingDocumentFromLookup(nil, fmt.Errorf("document 01KXJ9VHY3R14PZC7H56M3KH3A not found"))
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if found {
		t.Fatal("found = true, want false")
	}
}

func TestExistingDocumentFromLookup_EmptyDocIsRecoverable(t *testing.T) {
	// postgres/mysql/mariadb: empty struct with nil error
	_, found, err := existingDocumentFromLookup(&types.DefaultDocumentStructure{}, nil)
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if found {
		t.Fatal("found = true, want false")
	}
}

func TestExistingDocumentFromLookup_RealErrorPropagates(t *testing.T) {
	boom := errors.New("no such table: app_release_policy")
	_, found, err := existingDocumentFromLookup(nil, boom)
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want %v", err, boom)
	}
	if found {
		t.Fatal("found = true, want false")
	}
}

func TestExistingDocumentFromLookup_FoundDoc(t *testing.T) {
	doc, found, err := existingDocumentFromLookup(&types.DefaultDocumentStructure{
		ID:   "01KXJ9VHY3R14PZC7H56M3KH3A",
		Type: "app_release_policy",
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !found || doc == nil || doc.ID != "01KXJ9VHY3R14PZC7H56M3KH3A" {
		t.Fatalf("doc = %#v found = %v", doc, found)
	}
}

func TestIsDocumentNotFoundErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"sentinel", ae.ErrDocumentNotFound, true},
		{"wrapped sentinel", fmt.Errorf("lookup: %w", ae.ErrDocumentNotFound), true},
		{"sql no rows", sql.ErrNoRows, true},
		{"mongo/bbolt message", errors.New("document not found"), true},
		{"nil", nil, false},
		{"unrelated not found", errors.New("model not found"), false},
		{"other", errors.New("connection refused"), false},
	}
	for _, tc := range cases {
		if got := IsDocumentNotFoundErr(tc.err); got != tc.want {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

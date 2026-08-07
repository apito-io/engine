package resolver

import (
	"database/sql"
	"errors"
	"strings"

	ae "github.com/apito-io/engine/err"
	"github.com/apito-io/types"
)

// existingDocumentFromLookup interprets a GetSingleRawDocumentFromProject result.
//
// Project drivers disagree on how a missing document is reported: sqlite, mongo and
// bbolt return an error, while postgres, mysql and mariadb return an empty
// DefaultDocumentStructure with a nil error. Both mean "not there yet", which is
// recoverable for upsert semantics, so normalize them into found=false and reserve
// the returned error for genuine failures.
func existingDocumentFromLookup(raw interface{}, lookupErr error) (*types.DefaultDocumentStructure, bool, error) {
	if lookupErr != nil {
		if IsDocumentNotFoundErr(lookupErr) {
			return nil, false, nil
		}
		return nil, false, lookupErr
	}
	doc, ok := raw.(*types.DefaultDocumentStructure)
	if !ok || doc == nil || strings.TrimSpace(doc.ID) == "" {
		return nil, false, nil
	}
	return doc, true, nil
}

// IsDocumentNotFoundErr reports whether err means the document does not exist.
// Prefer ae.ErrDocumentNotFound in new driver code; the message fallback keeps
// drivers that still return ad-hoc errors working.
func IsDocumentNotFoundErr(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ae.ErrDocumentNotFound) || errors.Is(err, sql.ErrNoRows) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "document") && strings.Contains(msg, "not found")
}

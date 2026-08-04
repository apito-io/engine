package utility

import (
	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
)

// FieldIsSortable reports whether a top-level model field may appear in public
// GraphQL list sort payloads and be applied as SQL ORDER BY.
//
// Eligible: text, number, boolean, date, and single-choice fixed list (TEXT column).
// Not eligible: multiline, media, object, repeated, geo, multi-choice list.
func FieldIsSortable(f *models.FieldInfo) bool {
	if f == nil {
		return false
	}
	switch f.FieldType {
	case _const.TextField, _const.NumberField, _const.BooleanField, _const.DateField:
		return true
	case _const.ListField:
		if f.Validation == nil {
			return false
		}
		if f.Validation.IsMultiChoice {
			return false
		}
		return len(f.Validation.FixedListElements) > 0
	default:
		return false
	}
}

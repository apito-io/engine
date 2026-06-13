package sqlcommon

import (
	"context"
	"strings"

	"github.com/uptrace/bun"
)

// ExecAlterIgnoreDuplicate runs ALTER DDL and ignores duplicate-column errors.
func ExecAlterIgnoreDuplicate(ctx context.Context, db *bun.DB, sql string) error {
	_, err := db.ExecContext(ctx, sql)
	if err == nil || IsDuplicateColumnError(err) {
		return nil
	}
	return err
}

// IsDuplicateColumnError reports duplicate column DDL errors.
func IsDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "duplicate column name") {
		return true
	}
	if strings.Contains(msg, "already exists") && strings.Contains(msg, "column") {
		return true
	}
	return false
}

// IsSQLUniqueViolation reports primary/unique constraint violations across SQL engines.
func IsSQLUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique constraint") ||
		strings.Contains(s, "23505") ||
		strings.Contains(s, "duplicate entry")
}

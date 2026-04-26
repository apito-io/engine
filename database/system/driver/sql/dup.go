package sql

import (
	"strings"
)

// isSQLUniqueViolation reports whether err is a primary/unique constraint violation
// across SQLite, PostgreSQL, and MySQL (best-effort string matching).
func isSQLUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique constraint") ||
		strings.Contains(s, "23505") || // Postgres unique_violation
		strings.Contains(s, "duplicate entry") // MySQL 1062
}

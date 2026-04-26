package utility

import "strings"

// PostgresProjectSchemaName returns a safe PostgreSQL identifier for per-project schema isolation
// (GENERAL_POSTGRES_ISOLATION=schema). Prefix "p_" plus sanitized project id; max length 63 (PG limit).
func PostgresProjectSchemaName(projectID string) string {
	const prefix = "p_"
	const maxTotal = 63
	b := strings.Builder{}
	b.WriteString(prefix)
	for _, r := range strings.TrimSpace(projectID) {
		if b.Len() >= maxTotal {
			break
		}
		switch {
		case r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	s := b.String()
	if len(s) <= len(prefix) {
		return prefix + "project"
	}
	return s
}

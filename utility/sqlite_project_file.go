package utility

import "strings"

// SQLiteProjectFileName returns a filename under DefaultDatabaseDir for per-project SQLite files
// (GENERAL_SQLITE_FILE_PER_PROJECT). Must end with .sqlite for GetSQLDriver validation.
func SQLiteProjectFileName(projectID string) string {
	s := strings.TrimSpace(projectID)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "project"
	}
	if !strings.HasSuffix(strings.ToLower(out), ".sqlite") {
		out += ".sqlite"
	}
	return out
}

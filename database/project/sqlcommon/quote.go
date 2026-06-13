package sqlcommon

import "strings"

// QuotePGIdent returns a PostgreSQL double-quoted identifier for DDL.
func QuotePGIdent(s string) string {
	s = strings.ReplaceAll(s, `"`, `""`)
	return `"` + s + `"`
}

// QuoteMySQLIdent returns a MySQL backtick-quoted identifier for DDL.
func QuoteMySQLIdent(s string) string {
	s = strings.ReplaceAll(s, "`", "``")
	return "`" + s + "`"
}

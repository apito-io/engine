package sqlcommon

import "strings"

// IsAlreadyExistsErr reports duplicate database/schema errors during bootstrap DDL.
func IsAlreadyExistsErr(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") ||
		strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "database exists")
}

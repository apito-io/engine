package utility

import "github.com/oklog/ulid/v2"

// NewID returns a new lexicographically sortable identifier (ULID string, Crockford base32).
func NewID() string {
	return ulid.Make().String()
}

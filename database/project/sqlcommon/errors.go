package sqlcommon

import "fmt"

// ErrCredentialsRequired is returned when driver credentials are missing.
func ErrCredentialsRequired() error {
	return fmt.Errorf("driver credentials are required")
}

package interfaces

import "context"

// PhysicalTableColumnLister is an optional capability on ProjectDBInterface
// implementations. Used by read-only modelPhysicalHealth diagnostics.
type PhysicalTableColumnLister interface {
	// ListTableColumnNames returns physical column names for an existing table.
	// If the table does not exist, return an empty slice and nil error (or an
	// error that the caller treats as missing table). Prefer empty + nil when
	// the driver can distinguish missing tables cleanly.
	ListTableColumnNames(ctx context.Context, tableName string) ([]string, error)
}

package interfaces

import "context"

// ProjectReadOnlySQL is implemented by SQL project drivers.
// The public console uses it for local project exploration.
type ProjectReadOnlySQL interface {
	QueryReadOnly(ctx context.Context, query string, limit int) ([]string, [][]interface{}, error)
}

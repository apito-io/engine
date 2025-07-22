package utility

import (
	"context"
)

func BuildAsyncContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(ctx)
}

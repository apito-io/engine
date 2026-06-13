package schema

import "context"

type physicalDDLContextKey struct{}

// WithPhysicalDDLRequired annotates ctx when publish should flush project/tenant replicas.
func WithPhysicalDDLRequired(ctx context.Context, required bool) context.Context {
	return context.WithValue(ctx, physicalDDLContextKey{}, required)
}

// PhysicalDDLRequired reports whether project/tenant sync flush is needed after schema commit.
func PhysicalDDLRequired(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	v, ok := ctx.Value(physicalDDLContextKey{}).(bool)
	return ok && v
}

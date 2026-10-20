package mcp

import "context"

type restMetaKey struct{}

// RESTMeta is project scope and extra headers for an in-process REST call.
type RESTMeta struct {
	ProjectID string
	TenantID  string
	Headers   map[string]string
}

// WithRESTMeta attaches project scope and headers to the context used by System.REST.
func WithRESTMeta(ctx context.Context, projectID, tenantID string, headers map[string]string) context.Context {
	return context.WithValue(ctx, restMetaKey{}, RESTMeta{
		ProjectID: projectID,
		TenantID:  tenantID,
		Headers:   headers,
	})
}

// RESTMetaFrom reads metadata attached by WithRESTMeta.
func RESTMetaFrom(ctx context.Context) (RESTMeta, bool) {
	meta, ok := ctx.Value(restMetaKey{}).(RESTMeta)
	return meta, ok
}

package resolver

import (
	"context"

	"github.com/apito-io/engine/models"
)

// publicProjectDBContext returns the context used for GetProjectDriver and project DB operations
// on the public GraphQL surface. Pro SaaS hooks (PostApplicationCacheHook) attach per-tenant routing
// keys to cache.Ctx; the GraphQL resolve context (p.Context) may not carry those values, which would
// otherwise route to the shared project database instead of the tenant LibSQL URL.
func publicProjectDBContext(cache *models.ApplicationCache, requestCtx context.Context) context.Context {
	if cache != nil && cache.Ctx != nil {
		return cache.Ctx
	}
	return requestCtx
}

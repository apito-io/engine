package resolver

import (
	"context"

	"github.com/apito-io/engine/models"
)

// PublicProjectDBContext returns the context used for GetProjectDriver on project DB operations.
// Pro SaaS hooks attach tenant routing keys to cache.Ctx; prefer that over the raw request context.
func PublicProjectDBContext(cache *models.ApplicationCache, requestCtx context.Context) context.Context {
	if cache != nil && cache.Ctx != nil {
		return cache.Ctx
	}
	return requestCtx
}

func publicProjectDBContext(cache *models.ApplicationCache, requestCtx context.Context) context.Context {
	return PublicProjectDBContext(cache, requestCtx)
}

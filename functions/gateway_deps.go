package functions

import (
	"context"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

// GatewayDeps resolves project DB access for function data ops.
// Implemented by the engine/resolver layer so open-core stays free of driver imports
// beyond the interfaces package.
type GatewayDeps interface {
	// GetProjectDriver returns the project DB for the invocation's trusted DB context.
	GetProjectDriver(ctx context.Context, inv *InvocationContext) (interfaces.ProjectDBInterface, error)
	// ResolveModel maps a guest model name to a schema model.
	ResolveModel(inv *InvocationContext, modelArg string) (*models.ModelType, error)
	// NewParam clones the invocation param for a driver call.
	NewParam(inv *InvocationContext) *models.CommonSystemParams
}

package interfaces

import (
	"context"

	"github.com/apito-io/engine/models"
)

// FunctionLifecycleStore persists function revisions, deployments, and invocation summaries.
// Optional: SQL system drivers expose it via FunctionLifecycleStore(); others may return nil.
type FunctionLifecycleStore interface {
	SaveRevision(ctx context.Context, rev *models.FunctionRevision) error
	GetRevision(ctx context.Context, projectID, revisionID string) (*models.FunctionRevision, error)
	ListRevisions(ctx context.Context, projectID, functionName string, limit int) ([]*models.FunctionRevision, error)

	SaveBuild(ctx context.Context, build *models.FunctionBuild) error

	SaveDeployment(ctx context.Context, dep *models.FunctionDeployment) error
	ListDeployments(ctx context.Context, projectID, functionName string, limit int) ([]*models.FunctionDeployment, error)
	MarkDeploymentsSuperseded(ctx context.Context, projectID, functionName, exceptID string) error

	SaveInvocation(ctx context.Context, inv *models.FunctionInvocation) error
}

// FunctionLifecycleProvider is implemented by system drivers that own a lifecycle store.
type FunctionLifecycleProvider interface {
	FunctionLifecycleStore() FunctionLifecycleStore
}

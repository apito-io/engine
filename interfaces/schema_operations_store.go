package interfaces

import (
	"context"

	"github.com/apito-io/engine/models"
)

// SchemaOperationsStore persists schema change operation ledger rows.
// Implemented by SQL, MongoDB, and BBolt system drivers; optional for document-only paths.
type SchemaOperationsStore interface {
	CreateSchemaOperation(ctx context.Context, op *models.SchemaOperation) error
	UpdateSchemaOperation(ctx context.Context, op *models.SchemaOperation) error
	GetSchemaOperation(ctx context.Context, id string) (*models.SchemaOperation, error)
	ListSchemaOperationsByStatus(ctx context.Context, projectID string, statuses []string, limit int) ([]*models.SchemaOperation, error)
}

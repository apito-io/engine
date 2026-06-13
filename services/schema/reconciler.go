package schema

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

// ReconcileInput retries or reports repair for one ledger operation.
type ReconcileInput struct {
	Ctx           context.Context
	Op            *models.SchemaOperation
	BaseDriver    interfaces.ProjectDBInterface
	ApplyDDL      func(driver interfaces.ProjectDBInterface) error
	PersistSystem func() error
	RefreshCache  func() error
	Compensate    func(driver interfaces.ProjectDBInterface) error
}

// ReconcileOneOperation retries idempotent steps for failed / needs_repair / applying operations.
func ReconcileOneOperation(h Hooks, in ReconcileInput) (*models.SchemaOperation, error) {
	if h.Store == nil {
		return nil, fmt.Errorf("schema reconciler: store not available")
	}
	if in.Op == nil {
		return nil, fmt.Errorf("schema reconciler: operation required")
	}
	in.Op.AttemptCount++
	in.Op.Status = models.SchemaOpStatusApplying
	in.Op.UpdatedAt = utility.GetCurrentTime()
	if err := h.Store.UpdateSchemaOperation(in.Ctx, in.Op); err != nil {
		return nil, err
	}

	runErr := Run(h, RunInput{
		Ctx:           in.Ctx,
		Project:       &models.Project{ID: in.Op.ProjectID},
		OperationType: in.Op.OperationType,
		Request:       decodeRequest(in.Op.RequestJSON),
		BaseDriver:    in.BaseDriver,
		ApplyDDL:      in.ApplyDDL,
		PersistSystem: in.PersistSystem,
		RefreshCache:  in.RefreshCache,
		Compensate:    in.Compensate,
	})
	updated, err := h.Store.GetSchemaOperation(in.Ctx, in.Op.ID)
	if err != nil {
		return nil, err
	}
	if runErr != nil {
		return updated, runErr
	}
	return updated, nil
}

// ListRepairable returns operations that need reconciliation for a project.
func ListRepairable(ctx context.Context, store interfaces.SchemaOperationsStore, projectID string, limit int) ([]*models.SchemaOperation, error) {
	if store == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 50
	}
	return store.ListSchemaOperationsByStatus(ctx, projectID, []string{
		models.SchemaOpStatusApplying,
		models.SchemaOpStatusFailed,
		models.SchemaOpStatusNeedsRepair,
	}, limit)
}

func decodeRequest(raw string) interface{} {
	if raw == "" {
		return nil
	}
	var m map[string]interface{}
	if json.Unmarshal([]byte(raw), &m) == nil {
		return m
	}
	return raw
}

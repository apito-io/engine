package schema

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/apito-io/engine/interfaces"
	"github.com/apito-io/engine/models"
)

type memSchemaStore struct {
	mu   sync.Mutex
	byID map[string]*models.SchemaOperation
}

func newMemSchemaStore() *memSchemaStore {
	return &memSchemaStore{byID: make(map[string]*models.SchemaOperation)}
}

func (m *memSchemaStore) CreateSchemaOperation(ctx context.Context, op *models.SchemaOperation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[op.ID] = op
	return nil
}

func (m *memSchemaStore) UpdateSchemaOperation(ctx context.Context, op *models.SchemaOperation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.byID[op.ID] = op
	return nil
}

func (m *memSchemaStore) GetSchemaOperation(ctx context.Context, id string) (*models.SchemaOperation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byID[id], nil
}

func (m *memSchemaStore) ListSchemaOperationsByStatus(ctx context.Context, projectID string, statuses []string, limit int) ([]*models.SchemaOperation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*models.SchemaOperation
	for _, op := range m.byID {
		if projectID != "" && op.ProjectID != projectID {
			continue
		}
		out = append(out, op)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func TestApplyTenantDDL_NoHookIsNoOp(t *testing.T) {
	err := applyTenantDDL(Hooks{}, RunInput{
		Ctx:     context.Background(),
		Project: &models.Project{ID: "p1"},
		ApplyDDL: func(interfaces.ProjectDBInterface) error {
			return errors.New("should not run")
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestBeginOperation_RecordsApplying(t *testing.T) {
	store := newMemSchemaStore()
	project := &models.Project{ID: "p1", Schema: &models.ProjectSchema{}}
	op, err := beginOperation(store, RunInput{
		Ctx:           context.Background(),
		Project:       project,
		OperationType: models.SchemaOpTypeCreateModel,
		Request:       map[string]string{"name": "items"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if op.Status != models.SchemaOpStatusApplying {
		t.Fatalf("status=%s", op.Status)
	}
	steps, err := op.Steps()
	if err != nil || len(steps) != 4 {
		t.Fatalf("steps=%v err=%v", steps, err)
	}
}

func TestListRepairable_ReturnsFailedOps(t *testing.T) {
	store := newMemSchemaStore()
	_ = store.CreateSchemaOperation(context.Background(), &models.SchemaOperation{
		ID: "op1", ProjectID: "p1", OperationType: models.SchemaOpTypeAddField,
		Status: models.SchemaOpStatusNeedsRepair,
	})
	ops, err := ListRepairable(context.Background(), store, "p1", 10)
	if err != nil || len(ops) != 1 {
		t.Fatalf("ops=%v err=%v", ops, err)
	}
}

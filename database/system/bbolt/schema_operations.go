package bbolt

import (
	"context"
	"fmt"

	apitobolt "github.com/apito-io/apitoBolt"
	q "github.com/apito-io/apitoBolt/q"
	"github.com/apito-io/engine/models"
)

const schemaOperationsCollection = "schema_operations"

func (d *ProBBoltSystemDriver) CreateSchemaOperation(ctx context.Context, op *models.SchemaOperation) error {
	_ = ctx
	if d == nil || d.DB == nil || op == nil {
		return fmt.Errorf("create schema operation: invalid input")
	}
	if op.ID == "" {
		return fmt.Errorf("create schema operation: id required")
	}
	_, err := d.DB.Collection(schemaOperationsCollection).Save(op)
	return err
}

func (d *ProBBoltSystemDriver) UpdateSchemaOperation(ctx context.Context, op *models.SchemaOperation) error {
	_ = ctx
	if d == nil || d.DB == nil || op == nil || op.ID == "" {
		return fmt.Errorf("update schema operation: invalid input")
	}
	return d.DB.Collection(schemaOperationsCollection).Update(op)
}

func (d *ProBBoltSystemDriver) GetSchemaOperation(ctx context.Context, id string) (*models.SchemaOperation, error) {
	_ = ctx
	if id == "" {
		return nil, fmt.Errorf("get schema operation: id required")
	}
	var op models.SchemaOperation
	if err := d.DB.Collection(schemaOperationsCollection).FindByID(id, &op); err != nil {
		return nil, err
	}
	return &op, nil
}

func (d *ProBBoltSystemDriver) ListSchemaOperationsByStatus(ctx context.Context, projectID string, statuses []string, limit int) ([]*models.SchemaOperation, error) {
	_ = ctx
	if limit <= 0 {
		limit = 50
	}
	statusSet := make(map[string]struct{}, len(statuses))
	for _, s := range statuses {
		statusSet[s] = struct{}{}
	}
	collection := d.DB.Collection(schemaOperationsCollection)
	var query *apitobolt.Query
	if projectID != "" {
		query = collection.Select(q.Eq("project_id", projectID))
	} else {
		query = collection.Select()
	}
	var all []models.SchemaOperation
	if err := query.Find(&all); err != nil {
		return nil, err
	}
	var out []*models.SchemaOperation
	for i := range all {
		op := &all[i]
		if op == nil {
			continue
		}
		if projectID != "" && op.ProjectID != projectID {
			continue
		}
		if len(statusSet) > 0 {
			if _, ok := statusSet[op.Status]; !ok {
				continue
			}
		}
		out = append(out, op)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

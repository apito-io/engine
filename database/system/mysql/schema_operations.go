package mysql

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

func (d *Driver) CreateSchemaOperation(ctx context.Context, op *models.SchemaOperation) error {
	if d == nil || d.ORM == nil || op == nil {
		return fmt.Errorf("create schema operation: invalid input")
	}
	_, err := d.ORM.NewInsert().Model(op).Exec(ctx)
	return err
}

func (d *Driver) UpdateSchemaOperation(ctx context.Context, op *models.SchemaOperation) error {
	if d == nil || d.ORM == nil || op == nil || op.ID == "" {
		return fmt.Errorf("update schema operation: invalid input")
	}
	_, err := d.ORM.NewUpdate().Model(op).WherePK().Exec(ctx)
	return err
}

func (d *Driver) GetSchemaOperation(ctx context.Context, id string) (*models.SchemaOperation, error) {
	if id == "" {
		return nil, fmt.Errorf("get schema operation: id required")
	}
	op := &models.SchemaOperation{}
	err := d.ORM.NewSelect().Model(op).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return op, nil
}

func (d *Driver) ListSchemaOperationsByStatus(ctx context.Context, projectID string, statuses []string, limit int) ([]*models.SchemaOperation, error) {
	if limit <= 0 {
		limit = 50
	}
	var ops []*models.SchemaOperation
	q := d.ORM.NewSelect().Model(&ops).OrderExpr("updated_at DESC").Limit(limit)
	if projectID != "" {
		q = q.Where("project_id = ?", projectID)
	}
	if len(statuses) > 0 {
		q = q.Where("status IN (?)", bun.In(statuses))
	}
	if err := q.Scan(ctx); err != nil {
		return nil, err
	}
	return ops, nil
}

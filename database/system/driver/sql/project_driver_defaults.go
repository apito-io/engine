package sql

import (
	"context"

	"github.com/apito-io/engine/database/system/driverdefaults"
	"github.com/apito-io/engine/models"
)

// ensureProjectDriverCredentials inserts a driver_credentials row when the project exists and none is stored.
func (p *SystemSQLDriver) ensureProjectDriverCredentials(ctx context.Context, projectID string) error {
	if p == nil || p.ORM == nil || projectID == "" {
		return nil
	}
	projectExists, err := p.ORM.NewSelect().
		Model((*models.Project)(nil)).
		Where("id = ?", projectID).
		Exists(ctx)
	if err != nil || !projectExists {
		return err
	}
	hasDriver, err := p.ORM.NewSelect().
		Model((*models.DriverCredentials)(nil)).
		Where("project_id = ?", projectID).
		Exists(ctx)
	if err != nil {
		return err
	}
	if hasDriver {
		return nil
	}
	dc := driverdefaults.OSSBootstrapProjectDriver(p.Conf, projectID)
	if dc == nil {
		return nil
	}
	_, err = p.ORM.NewInsert().Model(dc).Exec(ctx)
	if err != nil && !isSQLUniqueViolation(err) {
		return err
	}
	return nil
}

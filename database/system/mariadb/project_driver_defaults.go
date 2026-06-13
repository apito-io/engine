package mariadb

import (
	"github.com/apito-io/engine/database/system/sqlcommon"
	"context"

	"github.com/apito-io/engine/database/system/driverdefaults"
	"github.com/apito-io/engine/models"
)

// ensureProjectDriverCredentials inserts a driver_credentials row when the project exists and none is stored.
func (d *Driver) ensureProjectDriverCredentials(ctx context.Context, projectID string) error {
	if d == nil || d.ORM == nil || projectID == "" {
		return nil
	}
	projectExists, err := d.ORM.NewSelect().
		Model((*models.Project)(nil)).
		Where("id = ?", projectID).
		Exists(ctx)
	if err != nil || !projectExists {
		return err
	}
	hasDriver, err := d.ORM.NewSelect().
		Model((*models.DriverCredentials)(nil)).
		Where("project_id = ?", projectID).
		Exists(ctx)
	if err != nil {
		return err
	}
	if hasDriver {
		return nil
	}
	dc := driverdefaults.OSSBootstrapProjectDriver(d.Conf, projectID)
	if dc == nil {
		return nil
	}
	_, err = d.ORM.NewInsert().Model(dc).Exec(ctx)
	if err != nil && !sqlcommon.IsSQLUniqueViolation(err) {
		return err
	}
	return nil
}

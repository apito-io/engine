package mysql

import (
	"context"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
)

func RunSQLiteLikePostDDL(context.Context, *Driver) error { return nil }

func RunAnalyzeAfterIndexDDL(context.Context, *Driver) {}

func PreflightSQLiteRelationParentTablesForAddRelation(ctx context.Context, db bun.IDB, engine string, from, to *models.ConnectionType) error {
	return nil
}

func (d *Driver) installRowCountTriggersForModel(context.Context, *models.ModelType) error { return nil }

func (d *Driver) installRowCountTriggersForModelTx(context.Context, bun.IDB, *models.ModelType) error {
	return nil
}

func (d *Driver) tryCountFromRowCountTable(context.Context, *models.CommonSystemParams) (int, bool, error) {
	return 0, false, nil
}

func dropAllTriggersForPhysicalTableTx(ctx context.Context, db bun.IDB, engine string, physicalTable string) error {
	return nil
}

func sqliteRebuildTableWithoutColumnTx(ctx context.Context, db bun.IDB, physicalTable, dropCol string) error {
	return nil
}

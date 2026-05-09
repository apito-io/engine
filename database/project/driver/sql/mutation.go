package sql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/uptrace/bun"
)

// ensureEnableIndexingForModelIDB creates indexes without ANALYZE (caller runs RunSQLiteLikePostDDL / RunAnalyzeAfterIndexDDL).
func (S *SQLDriver) ensureEnableIndexingForModelIDB(ctx context.Context, db bun.IDB, project *models.Project, model *models.ModelType) error {
	if model == nil {
		return nil
	}
	idxParam := &models.CommonSystemParams{Model: model}
	if project != nil {
		idxParam.ProjectID = project.ID
	}
	for _, f := range model.Fields {
		if f == nil || !f.EnableIndexing {
			continue
		}
		id := PhysicalSQLColumnForSystemRelationField(strings.TrimSpace(f.Identifier), model)
		if id == "" {
			continue
		}
		if err := S.execCreateIndexDDL(ctx, db, idxParam, id, ""); err != nil {
			return err
		}
	}
	return nil
}

// ensureEnableIndexingForModel creates secondary indexes for fields with EnableIndexing=true.
func (S *SQLDriver) ensureEnableIndexingForModel(ctx context.Context, project *models.Project, model *models.ModelType) error {
	if err := S.ensureEnableIndexingForModelIDB(ctx, S.ORM, project, model); err != nil {
		return err
	}
	RunAnalyzeAfterIndexDDL(ctx, S)
	return nil
}

func (S *SQLDriver) DropField(ctx context.Context, param *models.CommonSystemParams) error {

	if param.Model == nil {
		return errors.New("model is required")
	}
	if param.FieldInfo == nil {
		return errors.New("field info is required")
	}

	tableName := utility.PhysicalSQLTableName(param.Model.Name)
	sql := fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN %s;", tableName, param.FieldInfo.Identifier)
	_, err := S.ORM.NewRaw(sql).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (a *SQLDriver) RemoveAuthAddOns(ctx context.Context, project *models.Project, option map[string]interface{}) error {
	return nil
}

func (S *SQLDriver) TransferProject(ctx context.Context, userId, from, to string) error {
	return nil
}

type Meta2 struct {
	ID    string `gorm:"primaryKey;column:id;not null"`
	DocID string `gorm:"column:doc_id;not null"`

	CreatedAt time.Time `gorm:"column:created_at;not null;default:current_date"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null;default:current_date"`

	CreatedBy string `gorm:"column:created_by;not null"`

	UpdatedBy string `gorm:"column:updated_by;not null"`
	Status    string `gorm:"column:status;not null"`
}

// CreateTableOrCollection creates the physical table for param.Model only. Project bootstrap
// (database + meta + media) is handled by InitProjectBase.
func (S *SQLDriver) CreateTableOrCollection(ctx context.Context, param *models.CommonSystemParams, indexes []string) error {
	if param == nil || param.Model == nil {
		return nil
	}
	return S.CreateModelTable(ctx, param.Model, false)
}

func (a *SQLDriver) CheckTableOrCollectionExists(ctx context.Context, param *models.CommonSystemParams) (bool, error) {
	var query string
	var count int64

	table := strings.ReplaceAll(utility.PhysicalSQLTableName(param.Model.Name), "'", "''")
	switch a.DriverCredential.Engine {
	case _const.PostgreSQLDriver:
		query = fmt.Sprintf("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public' AND table_name = '%s'", table)
	case _const.MySQLDriver, _const.MariaDBDriver:
		query = fmt.Sprintf("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = '%s'", table)
	case _const.SQLiteDriver:
		query = fmt.Sprintf("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='%s'", table)
	default:
		return false, fmt.Errorf("unsupported database engine: %s", a.DriverCredential.Engine)
	}

	err := a.ORM.NewRaw(query).Scan(ctx, &count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func ddlBatchTxnSupported(engine string) bool {
	e := strings.ToLower(strings.TrimSpace(engine))
	return engineIsSQLiteLike(e) || e == _const.PostgreSQLDriver
}

func (S *SQLDriver) AddModel(ctx context.Context, project *models.Project, model *models.ModelType) (*models.ProjectSchema, error) {

	// if schema not found then create
	if project.Schema == nil {
		project.Schema = &models.ProjectSchema{
			Models: []*models.ModelType{
				model,
			},
		}
	} else {
		var found bool
		for _, ct := range project.Schema.Models {
			if ct.Name == model.Name {
				found = true
				break
			}
		}

		if !found {
			project.Schema.Models = append(project.Schema.Models, model)
		} else {
			return nil, errors.New("model Already Defined")
		}
	}

	type Fahim struct {
		Name string
	}

	eng := ""
	if S.DriverCredential != nil {
		eng = S.DriverCredential.Engine
	}
	if ddlBatchTxnSupported(eng) {
		err := S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			if err := S.createModelTableExec(ctx, tx, model, false); err != nil {
				return err
			}
			if err := S.ensureEnableIndexingForModelIDB(ctx, tx, project, model); err != nil {
				return err
			}
			return S.installRowCountTriggersForModelTx(ctx, tx, model)
		})
		if err != nil {
			return nil, err
		}
	} else {
		if err := S.CreateModelTable(ctx, model, false); err != nil {
			return nil, err
		}
		if err := S.ensureEnableIndexingForModel(ctx, project, model); err != nil {
			return nil, err
		}
		if err := S.installRowCountTriggersForModel(ctx, model); err != nil {
			return nil, err
		}
	}
	if err := RunSQLiteLikePostDDL(ctx, S); err != nil {
		return nil, err
	}

	return project.Schema, nil
}

// AddRelationFields creates a relation field (has one or has many) between models.
func (s *SQLDriver) AddRelationFields_AUTOGEN(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error {
	// Create pivot table for has_many relations or foreign key for has_one
	fromTable := utility.PhysicalSQLTableName(from.Model)
	toTable := utility.PhysicalSQLTableName(to.Model)

	if from.Relation == "has_many" {
		// Create pivot table
		pivotTable := fmt.Sprintf("%s_%s", fromTable, toTable)
		query := fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				id VARCHAR(36) PRIMARY KEY,
				%s_id VARCHAR(36),
				%s_id VARCHAR(36),
				created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (%s_id) REFERENCES %s(id),
				FOREIGN KEY (%s_id) REFERENCES %s(id)
			)
		`, pivotTable, fromTable, toTable, fromTable, fromTable, toTable, toTable)

		_, err := s.ORM.NewRaw(query).Exec(ctx)
		return err
	} else if from.Relation == "has_one" {
		// Add foreign key column
		query := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN %s_id VARCHAR(36)", toTable, fromTable)
		_, err := s.ORM.NewRaw(query).Exec(ctx)
		if err != nil {
			// Column might already exist, ignore error
			return nil
		}

		// Add foreign key constraint
		constraintQuery := fmt.Sprintf(
			"ALTER TABLE `%s` ADD CONSTRAINT fk_%s_%s FOREIGN KEY (%s_id) REFERENCES %s(id)",
			toTable, toTable, fromTable, fromTable, fromTable)
		s.ORM.NewRaw(constraintQuery).Exec(ctx) // Ignore errors for constraints
	}

	return nil
}

func (S *SQLDriver) AddRelationFields(ctx context.Context, from *models.ConnectionType, to *models.ConnectionType) error {

	if S.DriverCredential != nil {
		eng := strings.ToLower(strings.TrimSpace(S.DriverCredential.Engine))
		if err := PreflightSQLiteRelationParentTablesForAddRelation(ctx, S.ORM, eng, from, to); err != nil {
			return err
		}
	}

	toTableName := utility.PhysicalSQLTableName(from.Model)
	fromTableName := utility.PhysicalSQLTableName(to.Model)
	toFKColumn := relationFKColumnNameForModel(to.Model, from, to)
	fromFKColumn := relationFKColumnNameForModel(from.Model, from, to)
	pivotTable := relationPivotTableName(from, to)

	// Multi-step DDL must commit or roll back together on SQLite/libsql/Turso and PostgreSQL.
	runDDL := func(db bun.IDB) error {
		exec := func(q string) error {
			_, err := db.NewRaw(q).Exec(ctx)
			return err
		}

		// from connection
		switch from.Relation {
		case "has_one":
			switch to.Relation {
			case "has_one":
				// Symmetric 1:1 FK storage (matches synthetic system_<peer>_id on both models):
				//   toTableName (= from.Model) holds {toFieldName}_id → fromTableName(id)
				//   fromTableName (= to.Model) holds {fromFieldName}_id → toTableName(id)
				eng := strings.ToLower(strings.TrimSpace(S.DriverCredential.Engine))
				if engineIsSQLiteLike(eng) {
					q1 := fmt.Sprintf(
						"ALTER TABLE `%s` ADD %s VARCHAR(36) REFERENCES `%s` (id) ON DELETE CASCADE",
						toTableName, toFKColumn, fromTableName)
					if err := exec(q1); err != nil {
						return err
					}
					idx1 := fmt.Sprintf("idx_%s_%s_unique", toTableName, toFKColumn)
					if err := exec(fmt.Sprintf(
						"CREATE UNIQUE INDEX IF NOT EXISTS `%s` ON `%s` (`%s`)",
						idx1, toTableName, toFKColumn)); err != nil {
						return err
					}
					q2 := fmt.Sprintf(
						"ALTER TABLE `%s` ADD %s VARCHAR(36) REFERENCES `%s` (id) ON DELETE CASCADE",
						fromTableName, fromFKColumn, toTableName)
					if err := exec(q2); err != nil {
						return err
					}
					idx2 := fmt.Sprintf("idx_%s_%s_unique", fromTableName, fromFKColumn)
					return exec(fmt.Sprintf(
						"CREATE UNIQUE INDEX IF NOT EXISTS `%s` ON `%s` (`%s`)",
						idx2, fromTableName, fromFKColumn))
				}
				if eng == _const.PostgreSQLDriver {
					q1 := fmt.Sprintf(
						"ALTER TABLE %s ADD COLUMN %s VARCHAR(36) UNIQUE REFERENCES %s (id) ON DELETE CASCADE",
						QuotePGIdent(toTableName), QuotePGIdent(toFKColumn), QuotePGIdent(fromTableName))
					if err := exec(q1); err != nil {
						return err
					}
					q2 := fmt.Sprintf(
						"ALTER TABLE %s ADD COLUMN %s VARCHAR(36) UNIQUE REFERENCES %s (id) ON DELETE CASCADE",
						QuotePGIdent(fromTableName), QuotePGIdent(fromFKColumn), QuotePGIdent(toTableName))
					return exec(q2)
				}
				q1 := fmt.Sprintf(
					"ALTER TABLE `%s` ADD %s VARCHAR(36) UNIQUE REFERENCES `%s` (id) ON DELETE CASCADE",
					toTableName, toFKColumn, fromTableName)
				if err := exec(q1); err != nil {
					return err
				}
				q2 := fmt.Sprintf(
					"ALTER TABLE `%s` ADD %s VARCHAR(36) UNIQUE REFERENCES `%s` (id) ON DELETE CASCADE",
					fromTableName, fromFKColumn, toTableName)
				return exec(q2)
			case "has_many":
				// FK column on fromTableName pointing at toTableName (e.g. chef.tenant_id → tenant.id).
				// SQLite/libsql: only one ADD per ALTER — use inline REFERENCES, not comma + ADD CONSTRAINT.
				// MySQL/MariaDB/SQLite/libsql: backtick-quoted identifiers. PostgreSQL: double-quoted (QuotePGIdent).
				col := fromFKColumn
				eng := strings.ToLower(strings.TrimSpace(S.DriverCredential.Engine))
				var query string
				if eng == _const.PostgreSQLDriver {
					query = fmt.Sprintf(
						"ALTER TABLE %s ADD COLUMN %s VARCHAR(36) REFERENCES %s (id) ON DELETE CASCADE",
						QuotePGIdent(fromTableName), QuotePGIdent(col), QuotePGIdent(toTableName))
				} else {
					ft := strings.ReplaceAll(fromTableName, "`", "``")
					tt := strings.ReplaceAll(toTableName, "`", "``")
					query = fmt.Sprintf(
						"ALTER TABLE `%s` ADD COLUMN %s VARCHAR(36) REFERENCES `%s` (id) ON DELETE CASCADE",
						ft, col, tt)
				}
				return exec(query)
			}
		case "has_many":
			switch to.Relation {
			case "has_many":
				// Pivot table name + columns use lexicographic order of physical model slugs (see relationPivotTableNameParts).
				pa, pb := sortedPhysicalPair(from.Model, to.Model)
				paq := strings.ReplaceAll(pa, "`", "``")
				pbq := strings.ReplaceAll(pb, "`", "``")
				query := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS `+"`%s`"+`(
				`+"`%s_id`"+` VARCHAR(36) REFERENCES `+"`%s`"+` (id) ON DELETE CASCADE,
				`+"`%s_id`"+` VARCHAR(36) REFERENCES `+"`%s`"+` (id) ON DELETE CASCADE,
				PRIMARY KEY (`+"`%s_id`, `%s_id`"+`)
			);`, pivotTable,
					paq, paq, pbq, pbq,
					paq, pbq)
				if err := exec(query); err != nil {
					return err
				}
				pivotName := pivotTable
				escPivot := strings.ReplaceAll(pivotName, "`", "``")
				idxA := fmt.Sprintf("CREATE INDEX IF NOT EXISTS `idx_%s_%s_id` ON `%s` (`%s_id`)", escPivot, paq, escPivot, paq)
				idxB := fmt.Sprintf("CREATE INDEX IF NOT EXISTS `idx_%s_%s_id` ON `%s` (`%s_id`)", escPivot, pbq, escPivot, pbq)
				if err := exec(idxA); err != nil {
					return err
				}
				return exec(idxB)
			case "has_one":
				//same for one to one & one to many
				query := fmt.Sprintf("ALTER TABLE `%s` ADD %s VARCHAR(36) REFERENCES `%s` (id) ON DELETE CASCADE;", toTableName, toFKColumn, fromTableName)
				return exec(query)
			}
		}
		return nil
	}
	err := S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return runDDL(tx)
	})
	if err != nil {
		return err
	}

	if S.DriverCredential != nil && engineIsSQLiteLike(strings.ToLower(strings.TrimSpace(S.DriverCredential.Engine))) {
		return RunSQLiteLikePostDDL(ctx, S)
	}
	return nil
}

// DeleteRelationDocuments drops pivot tables, relation keys, or collection tables and all documents within them.
func (s *SQLDriver) DeleteRelationDocuments_AUTOGEN(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error {
	fromTable := utility.PhysicalSQLTableName(from.Model)
	toTable := utility.PhysicalSQLTableName(to.Model)

	if from.Relation == "has_many" {
		// Drop pivot table
		pivotTable := fmt.Sprintf("%s_%s", fromTable, toTable)
		query := fmt.Sprintf("DROP TABLE IF EXISTS `%s`", pivotTable)
		_, err := s.ORM.NewRaw(query).Exec(ctx)
		return err
	} else if from.Relation == "has_one" {
		// Remove foreign key column
		query := fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN %s_id", toTable, fromTable)
		_, err := s.ORM.NewRaw(query).Exec(ctx)
		return err
	}

	return nil
}

// sqliteLikeExecDropColumnTx runs ALTER TABLE DROP COLUMN with Turso/libsql/SQLite fallbacks, then table rebuild.
// Turso MCP on rosna tenant DB: sqlite_master showed 0 triggers on employee; CREATE TABLE mixed
// inline role_id REFERENCES with a separate CONSTRAINT on tenant_id — plain DROP COLUMN fails (matches stock SQLite).
func sqliteLikeExecDropColumnTx(ctx context.Context, db bun.IDB, physicalTable, column string) error {
	tq := strings.ReplaceAll(physicalTable, "`", "``")
	cq := strings.ReplaceAll(column, "`", "``")
	alterSQL := fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", tq, cq)
	tryAlter := func() error {
		_, err := db.NewRaw(alterSQL).Exec(ctx)
		return err
	}
	firstErr := tryAlter()
	if firstErr == nil {
		return nil
	}
	if _, err := db.NewRaw(`PRAGMA legacy_alter_table = ON`).Exec(ctx); err == nil {
		err2 := tryAlter()
		_, _ = db.NewRaw(`PRAGMA legacy_alter_table = OFF`).Exec(ctx)
		if err2 == nil {
			return nil
		}
	}
	// This must run outside an active transaction. SQLite ignores PRAGMA foreign_keys changes inside
	// transactions, which would force FK validation during the rebuild and strip unrelated constraints.
	if _, err := db.NewRaw(`PRAGMA foreign_keys = OFF`).Exec(ctx); err != nil {
		return firstErr
	}
	err3 := tryAlter()
	if err3 == nil {
		_, _ = db.NewRaw(`PRAGMA foreign_keys = ON`).Exec(ctx)
		return nil
	}
	rebuildErr := sqliteRebuildTableWithoutColumnTx(ctx, db, physicalTable, column)
	_, _ = db.NewRaw(`PRAGMA foreign_keys = ON`).Exec(ctx)
	if rebuildErr != nil {
		return fmt.Errorf("%w (sqlite rebuild drop column: %v)", err3, rebuildErr)
	}
	return nil
}

func (S *SQLDriver) DeleteRelationDocuments(ctx context.Context, projectId string, from *models.ConnectionType, to *models.ConnectionType) error {
	if S.DriverCredential == nil {
		return errors.New("sql driver: missing credentials")
	}
	if from == nil || to == nil {
		return errors.New("connection types required")
	}

	eng := strings.ToLower(strings.TrimSpace(S.DriverCredential.Engine))
	sqliteLike := engineIsSQLiteLike(eng)
	pg := eng == _const.PostgreSQLDriver
	mysqlLike := eng == _const.MySQLDriver || eng == _const.MariaDBDriver

	toTableName := utility.PhysicalSQLTableName(from.Model)
	fromTableName := utility.PhysicalSQLTableName(to.Model)
	toFKColumn := relationFKColumnNameForModel(to.Model, from, to)
	fromFKColumn := relationFKColumnNameForModel(from.Model, from, to)
	pivotTable := relationPivotTableName(from, to)

	runDDL := func(db bun.IDB) error {
		exec := func(q string) error {
			_, err := db.NewRaw(q).Exec(ctx)
			return err
		}

		// Drop a foreign-key column. SQLite/libsql/Turso: only DROP COLUMN (3.35+; FK dropped with column).
		// MySQL/MariaDB: DROP FOREIGN KEY then DROP COLUMN as separate statements (no multi-statement batch).
		// PostgreSQL: DROP COLUMN ... CASCADE.
		// sqliteLogicalModel: schema model name for this physical table — used to drop/reinstall Apito row-count
		// triggers before/after DROP COLUMN (triggers may reference dropped cols; SQLite rebuild then fails).
		dropFKColumn := func(table, col, mysqlFKName string, sqliteLogicalModel string) error {
			tq := strings.ReplaceAll(table, "`", "``")
			cq := strings.ReplaceAll(col, "`", "``")
			if sqliteLike {
				// Drop every trigger on this table (not only Apito row-count): any trigger body that
				// references the FK column breaks SQLite DROP COLUMN rebuild if env flags differ from DB state.
				if err := dropAllTriggersForPhysicalTableTx(ctx, db, eng, table); err != nil {
					return err
				}
				// Dual has_one/has_one adds CREATE UNIQUE INDEX idx_<table>_<peer>_id_unique (see AddRelationFields).
				// SQLite DROP COLUMN fails if that index still references the column ("error in index ... after drop column").
				if strings.TrimSpace(col) != "" {
					idx := fmt.Sprintf("idx_%s_%s_unique", table, col)
					iq := strings.ReplaceAll(idx, "`", "``")
					if err := exec(fmt.Sprintf("DROP INDEX IF EXISTS `%s`", iq)); err != nil {
						return err
					}
				}
				if err := sqliteLikeExecDropColumnTx(ctx, db, table, col); err != nil {
					return err
				}
				if strings.TrimSpace(sqliteLogicalModel) != "" {
					return S.installRowCountTriggersForModelTx(ctx, db, &models.ModelType{Name: sqliteLogicalModel})
				}
				return nil
			}
			if pg {
				return exec(fmt.Sprintf(
					"ALTER TABLE %s DROP COLUMN %s CASCADE",
					QuotePGIdent(table), QuotePGIdent(col)))
			}
			if mysqlLike && mysqlFKName != "" {
				fkq := strings.ReplaceAll(mysqlFKName, "`", "``")
				if err := exec(fmt.Sprintf("ALTER TABLE `%s` DROP FOREIGN KEY `%s`", tq, fkq)); err != nil {
					return err
				}
			}
			return exec(fmt.Sprintf("ALTER TABLE `%s` DROP COLUMN `%s`", tq, cq))
		}

		var txErr error
		switch from.Relation {
		case "has_one":
			switch to.Relation {
			case "has_one":
				col := toFKColumn
				fk := fmt.Sprintf("fk_%s_%s", toTableName, toFKColumn)
				if txErr = dropFKColumn(toTableName, col, fk, from.Model); txErr != nil {
					break
				}
				col2 := fromFKColumn
				fk2 := fmt.Sprintf("fk_%s_%s", fromTableName, fromFKColumn)
				txErr = dropFKColumn(fromTableName, col2, fk2, to.Model)
			case "has_many":
				col := fromFKColumn
				fk := fmt.Sprintf("fk_%s_%s", fromTableName, fromFKColumn)
				txErr = dropFKColumn(fromTableName, col, fk, to.Model)
			}
		case "has_many":
			switch to.Relation {
			case "has_many":
				pivot := pivotTable
				if pg {
					txErr = exec(fmt.Sprintf("DROP TABLE IF EXISTS %s", QuotePGIdent(pivot)))
				} else {
					pq := strings.ReplaceAll(pivot, "`", "``")
					txErr = exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", pq))
				}
			case "has_one":
				col := toFKColumn
				fk := fmt.Sprintf("fk_%s_%s", toTableName, toFKColumn)
				txErr = dropFKColumn(toTableName, col, fk, from.Model)
			}
		}
		return txErr
	}

	var err error
	if sqliteLike {
		// SQLite/libsql requires PRAGMA foreign_keys=OFF for rebuild-with-FK-copy. The PRAGMA is
		// ignored inside transactions and connection-local; SQLite/libsql handles are configured
		// with MaxOpenConns(1), so run directly on the tenant/project DB handle.
		err = runDDL(S.ORM)
	} else {
		err = S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
			return runDDL(tx)
		})
	}
	if err != nil {
		return err
	}
	return RunSQLiteLikePostDDL(ctx, S)
}

func (S *SQLDriver) AddFieldToModel(ctx context.Context, param *models.CommonSystemParams, isUpdate bool, parent_field string) (*models.ModelType, error) {

	if param.FieldInfo.InputType == "geo" {
		return nil, errors.New("geo Field is not supported in PostgreSQL. We will be integrating it soon")
	}

	if parent_field == "" && !isUpdate {
		if (!isUpdate && param.FieldInfo.Serial == 0) || len(param.Model.Fields) == 0 { // new field cant be zero
			param.FieldInfo.Serial = uint32(len(param.Model.Fields) + 1)
		}
		param.Model.Fields = append(param.Model.Fields, param.FieldInfo)
	} else if parent_field != "" {
		for _, f := range param.Model.Fields {
			if f.Identifier == parent_field {
				subField := param.FieldInfo
				var found bool
				for i, s := range f.SubFieldInfo {
					if s.Identifier == param.FieldInfo.Identifier {
						f.SubFieldInfo[i] = subField
						found = true
						break
					}
				}
				if !found {
					subField.Serial = uint32(len(f.SubFieldInfo)) + 1
					f.SubFieldInfo = append(f.SubFieldInfo, subField)
				}
			}
		}
	}

	if parent_field != "" {
		return param.Model, nil // dont create anything
		// todo transform this to one to many relation
	}

	// Root-level metadata updates: resolver already merges FieldInfo into param.Model.Fields.
	// Avoid ALTER TABLE ADD COLUMN / index DDL here — dedicated mutations handle column lifecycle.
	if parent_field == "" && isUpdate {
		return param.Model, nil
	}

	tableName := utility.PhysicalSQLTableName(param.Model.Name)
	stmts, err := AlterTableAddFieldSQL(S.DriverCredential.Engine, tableName, param.FieldInfo)
	if err != nil {
		return nil, err
	}
	err = S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		for _, query := range stmts {
			if _, err := tx.NewRaw(query).Exec(ctx); err != nil {
				return err
			}
		}
		if parent_field == "" && !isUpdate && param.FieldInfo.EnableIndexing {
			idxParam := &models.CommonSystemParams{Model: param.Model, ProjectID: param.ProjectID}
			return S.execCreateIndexDDL(ctx, tx, idxParam, param.FieldInfo.Identifier, "")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := RunSQLiteLikePostDDL(ctx, S); err != nil {
		return nil, err
	}

	return param.Model, nil
}

func (S *SQLDriver) AddATeamMemberToProject(ctx context.Context, req *models.TeamMemberAddRequest) error {
	panic("add team member to project not implemented")
}

func (S *SQLDriver) RemoveATeamMemberFromProject(ctx context.Context, projectId string, memberId string) error {
	panic("remove a team member from project not implemented")
}

func (S *SQLDriver) CreateMediaDocument(ctx context.Context, projectId string, media *models.FileDetails) (*models.FileDetails, error) {

	data := map[string]interface{}{
		"id": media.ID,
	}
	if media.UploadParam != nil {
		if media.UploadParam.ModelName != "" {
			data["model"] = media.UploadParam.ModelName
		}
		if media.UploadParam.FieldName != "" {
			data["field"] = media.UploadParam.FieldName
		}
	}
	if media.ContentType != "" {
		data["media_type"] = media.ContentType
	}
	if media.FileExtension != "" {
		data["file_extension"] = media.FileExtension
	}
	if media.FileName != "" {
		data["file_name"] = media.FileName
	}
	if media.Size != 0 {
		data["size"] = media.Size
	}
	if media.S3Key != "" {
		data["s3_key"] = media.S3Key
	}
	if media.URL != "" {
		data["url"] = media.URL
	}

	_, err := S.ORM.NewInsert().Table("media").Model(&data).Exec(ctx)
	if err != nil {
		return nil, err
	}

	return media, nil
}

func (S *SQLDriver) AddDocumentToProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure) (interface{}, error) {

	err := S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {

		tableName := utility.PhysicalSQLTableName(param.Model.Name)

		data := map[string]interface{}{
			"id": doc.ID,
		}
		for k, v := range doc.Data {
			if val, ok := v.(map[string]interface{}); ok {
				if html, ok := val["html"]; ok {
					data[k] = html
				}
			} else {
				data[k] = v
			}
		}
		if err := runDocumentPreInsertHook(S.Conf, ctx, param, data); err != nil {
			return err
		}
		if err := runDocumentPreInsertDocHook(S.Conf, ctx, param, doc); err != nil {
			return err
		}
		mergeDocumentTaggedFieldsIntoData(doc, data)
		StripArangoEnvelopeKeysForSQLInsert(data, param)
		remapSyntheticSystemRelationRowKeys(data, param.Model)
		_, err := tx.NewInsert().Table(tableName).Model(&data).Exec(ctx)
		if err != nil {
			return err
		}

		// now insert a meta data
		metaData := map[string]interface{}{
			"id":         utility.NewID(),
			"created_by": doc.Meta.CreatedBy.ID,
			"updated_by": doc.Meta.LastModifiedBy.ID,
			"status":     doc.Meta.Status,
			"doc_id":     doc.ID,
		}
		_, err = tx.NewInsert().Table("meta").Model(&metaData).Exec(ctx)
		if err != nil {
			return err
		}

		// return nil will commit the whole transaction
		return nil
	})
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func (S *SQLDriver) UpdateDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, doc *types.DefaultDocumentStructure, replace bool) error {

	var multilineFields []string
	var pictureField []string
	var galleryField []string
	var listFields []string
	var repeatedFields []string
	for _, f := range param.Model.Fields {
		if f.FieldType == _const.MultilineField {
			multilineFields = append(multilineFields, f.Identifier)
		} else if f.FieldType == _const.MediaField && f.Validation != nil && f.Validation.IsGallery {
			galleryField = append(galleryField, f.Identifier)
		} else if f.FieldType == _const.MediaField && f.Validation != nil && !f.Validation.IsGallery {
			pictureField = append(pictureField, f.Identifier)
		} else if f.FieldType == _const.ListField && f.Validation != nil && (len(f.Validation.FixedListElements) == 0 || f.Validation.IsMultiChoice) {
			listFields = append(listFields, f.Identifier)
		} else if f.FieldType == _const.RepeatedField {
			repeatedFields = append(repeatedFields, f.Identifier)
		}
	}

	tableName := utility.PhysicalSQLTableName(doc.Type)

	data := map[string]interface{}{}
	for k, v := range doc.Data {
		// if it's a map then it must be a media field
		kind := reflect.ValueOf(v).Kind()
		switch kind {
		case reflect.String, reflect.Int, reflect.Int64, reflect.Float64, reflect.Bool, reflect.Int32:
			data[k] = v
		case reflect.Map:
			val := v.(map[string]interface{})
			if utility.ArrayContains(multilineFields, k) {
				if html, ok := val["html"]; ok {
					data[k] = html
				}
			} else if utility.ArrayContains(pictureField, k) {
				b, _ := json.Marshal(v)
				data[k] = string(b)
			} else {
				// a map can be a object field like address, contact, etc.
				data[k] = val
			}
		case reflect.Ptr:
			fmt.Println(v)
		case reflect.Slice:
			if utility.ArrayContains(galleryField, k) || utility.ArrayContains(listFields, k) || utility.ArrayContains(repeatedFields, k) {
				b, err := json.Marshal(v)
				if err != nil {
					return err
				}
				data[k] = string(b)
			}
		case reflect.Invalid:
			data[k] = nil
		case reflect.Struct:
			data[k] = v
		default:
			panic("unhandled default case")
		}

	}
	remapSyntheticSystemRelationRowKeys(data, param.Model)
	return S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		q := tx.NewUpdate().Table(tableName).Where("id = ?", doc.ID)
		q = applyBunHookWheresUpdate(S.Conf, ctx, param, q)
		if _, err := q.Model(&data).Exec(ctx); err != nil {
			return err
		}
		metaData := map[string]interface{}{
			"updated_at": utility.GetCurrentTime(),
			"updated_by": param.UserID,
		}
		_, err := tx.NewUpdate().Table("meta").Where("doc_id = ?", doc.ID).Model(&metaData).Exec(ctx)
		return err
	})
}

func (S *SQLDriver) DeleteDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) error {

	tableName := utility.PhysicalSQLTableName(param.Model.Name)

	return S.ORM.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.NewDelete().Table("document_revisions").
			Where("original_doc_id = ? OR id = ?", param.DocumentID, param.DocumentID).
			Exec(ctx); err != nil {
			return err
		}
		if _, err := tx.NewDelete().Table("meta").Where("doc_id = ?", param.DocumentID).Exec(ctx); err != nil {
			return err
		}
		q := tx.NewDelete().Table(tableName).Where("id = ?", param.DocumentID)
		q = applyBunHookWheresDelete(S.Conf, ctx, param, q)
		_, err := q.Exec(ctx)
		return err
	})
}

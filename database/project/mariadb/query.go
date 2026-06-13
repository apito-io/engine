package mariadb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	_const "github.com/apito-io/engine/const"
	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
	"github.com/apito-io/types"
	"github.com/tailor-platform/graphql"
	"github.com/uptrace/bun"
)

func (d *Driver) maybeEnsureRelationDDL(ctx context.Context, param *models.CommonSystemParams) error {
	if d == nil || param == nil || param.ResolveParams == nil {
		return nil
	}
	conn, ok := param.ResolveParams.Args["connection"].(map[string]interface{})
	if !ok || len(conn) == 0 {
		return nil
	}
	if len(param.ProjectSchemaModels) == 0 {
		return nil
	}
	return d.EnsureRelationArtifactsFromSchema(ctx, param.ProjectSchemaModels)
}

func (d *Driver) CountDocOfProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	// Must use RootResolverQueryBuilder(..., true): RootConnectionResolverQueryBuilder emits Arango AQL (FOR/FILTER/...), not SQL.
	if n, ok, err := d.tryCountFromRowCountTable(ctx, param); ok {
		return int(n), err
	}
	if err := d.maybeEnsureRelationDDL(ctx, param); err != nil {
		return nil, err
	}
	query, args, err := RootResolverQueryBuilder(d.Conf, param, true)
	if err != nil {
		return nil, err
	}

	var result int64
	var scanErr error
	if len(args) > 0 {
		scanErr = d.ORM.NewRaw(query, args...).Scan(ctx, &result)
	} else {
		scanErr = d.ORM.NewRaw(query).Scan(ctx, &result)
	}
	if scanErr != nil {
		return nil, fmt.Errorf("failed to execute SQL:\n%w", scanErr)
	}

	return int(result), nil
}

func (d *Driver) CountDocOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	n, err := d.CountDocOfProject(ctx, param)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]interface{}{"total": n})
}

func (d *Driver) AddAuthAddOns(ctx context.Context, project *models.Project, auth map[string]interface{}) error {
	panic("add auth addons not implemented")
}

// pivotManyManyInsertRow builds column names for a pivot INSERT. ForwardConnectionID is always the
// document being updated; relatedID is the other endpoint (ConnectDisconnectParamBuilder).
func pivotManyManyInsertRow(cdp *models.ConnectDisconnectParam, relatedID string) map[string]interface{} {
	if cdp.ForwardConnectionModelType != nil && cdp.BackwardConnectionModelType != nil {
		return map[string]interface{}{
			fmt.Sprintf(`%s_id`, utility.PhysicalSQLTableName(cdp.ForwardConnectionModelType.Name)):  cdp.ForwardConnectionID,
			fmt.Sprintf(`%s_id`, utility.PhysicalSQLTableName(cdp.BackwardConnectionModelType.Name)): relatedID,
		}
	}
	return map[string]interface{}{
		fmt.Sprintf(`%s_id`, utility.PhysicalSQLTableName(cdp.BackwardConnectionType.Model)): cdp.ForwardConnectionID,
		fmt.Sprintf(`%s_id`, utility.PhysicalSQLTableName(cdp.ForwardConnectionType.Model)):  relatedID,
	}
}

// humanizeSQLModelName turns snake_case table/model ids into spaced words for log readability.
func humanizeSQLModelName(model string) string {
	if model == "" {
		return ""
	}
	return strings.ReplaceAll(utility.PhysicalSQLTableName(model), "_", " ")
}

func connectBuilderKnownAsSuffix(cdp *models.ConnectDisconnectParam) string {
	if cdp == nil {
		return ""
	}
	k := cdp.KnownAs
	if k == "" && cdp.ForwardConnectionType != nil {
		k = cdp.ForwardConnectionType.KnownAs
	}
	if k == "" {
		return ""
	}
	return fmt.Sprintf(` known_as=%q`, k)
}

func connectBuilderSchemaArrow(cdp *models.ConnectDisconnectParam) string {
	if cdp == nil || cdp.ForwardConnectionType == nil || cdp.BackwardConnectionType == nil {
		return "(incomplete schema)"
	}
	fm := utility.PhysicalSQLTableName(cdp.ForwardConnectionType.Model)
	bm := utility.PhysicalSQLTableName(cdp.BackwardConnectionType.Model)
	return fmt.Sprintf("%s (%s) → %s (%s)%s",
		fm, cdp.ForwardConnectionType.Relation,
		bm, cdp.BackwardConnectionType.Relation,
		connectBuilderKnownAsSuffix(cdp))
}

// connectBuilderCardinalitySentence describes the forward/backward pair in plain English.
func connectBuilderCardinalitySentence(cdp *models.ConnectDisconnectParam) string {
	if cdp == nil || cdp.ForwardConnectionType == nil || cdp.BackwardConnectionType == nil {
		return ""
	}
	fm := humanizeSQLModelName(cdp.ForwardConnectionType.Model)
	bm := humanizeSQLModelName(cdp.BackwardConnectionType.Model)
	fr, br := cdp.ForwardConnectionType.Relation, cdp.BackwardConnectionType.Relation
	switch {
	case fr == "has_one" && br == "has_one":
		return fmt.Sprintf("Each %s has at most one %s, and each %s has at most one %s (one-to-one).",
			fm, bm, bm, fm)
	case fr == "has_one" && br == "has_many":
		return fmt.Sprintf("Each %s belongs to one %s; each %s has many %s.",
			bm, fm, fm, bm)
	case fr == "has_many" && br == "has_one":
		return fmt.Sprintf("Each %s belongs to one %s; each %s has many %s.",
			fm, bm, bm, fm)
	case fr == "has_many" && br == "has_many":
		return fmt.Sprintf("Many-to-many between %s and %s (pivot table).", fm, bm)
	default:
		return fmt.Sprintf("%s is %s on the forward side; %s is %s on the backward side.", fm, fr, bm, br)
	}
}

// connectBuilderConnectNarrative is a compact multi-line summary for logs (schema + cardinality + ids).
func connectBuilderConnectNarrative(cdp *models.ConnectDisconnectParam, actionID string) string {
	if cdp == nil {
		return "connect: (nil param)"
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("connection_type=%s\n", cdp.ConnectionType))
	b.WriteString("edge: ")
	b.WriteString(connectBuilderSchemaArrow(cdp))
	b.WriteByte('\n')
	if s := connectBuilderCardinalitySentence(cdp); s != "" {
		b.WriteString("summary: ")
		b.WriteString(s)
		b.WriteByte('\n')
	}
	b.WriteString(fmt.Sprintf("ids: forward_connection_id=%s action_id=%s (all action_ids=%v)",
		cdp.ForwardConnectionID, actionID, cdp.ActionIDs))
	return b.String()
}

func logConnectBuilderBegin(narrative string) {
	var b strings.Builder
	b.WriteString("[sqldriver.ConnectBuilder] begin connect\n")
	for _, line := range strings.Split(narrative, "\n") {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	log.Print(b.String())
}

// formatBunUpdateSQL returns the parameterized SQL bun would run for an UPDATE (after Model/Where/Set chain).
func formatBunUpdateSQL(qu *bun.UpdateQuery) (string, error) {
	if qu == nil {
		return "", errors.New("nil update query")
	}
	db := qu.DB()
	if db == nil {
		return "", errors.New("nil DB on update query")
	}
	b, err := qu.AppendQuery(db.Formatter(), nil)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (d *Driver) logConnectBuilderSQL(narrative, step, sql string, args []interface{}) {
	eng := ""
	if d != nil && d.DriverCredential != nil {
		eng = strings.TrimSpace(d.DriverCredential.Engine)
	}
	var b strings.Builder
	b.WriteString("[sqldriver.ConnectBuilder] SQL\n")
	if narrative != "" {
		for _, line := range strings.Split(narrative, "\n") {
			b.WriteString("  ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if step != "" {
		b.WriteString("  step: ")
		b.WriteString(step)
		b.WriteByte('\n')
	}
	b.WriteString("  engine: ")
	b.WriteString(eng)
	b.WriteString("\n  SQL: ")
	b.WriteString(sql)
	if len(args) > 0 {
		b.WriteString("\n  args: ")
		fmt.Fprintf(&b, "%v", args)
	}
	b.WriteByte('\n')
	log.Print(b.String())
}

func (d *Driver) logBunUpdateQuery(narrative, step string, qu *bun.UpdateQuery) {
	sql, err := formatBunUpdateSQL(qu)
	if err != nil {
		log.Printf("[sqldriver.ConnectBuilder] step=%s format sql: %v", step, err)
		return
	}
	d.logConnectBuilderSQL(narrative, step, sql, nil)
}

// execPivotInsertIgnoreDuplicate inserts one M:N pivot row; duplicate primary keys are ignored so
// update mutations can safely resend the same connect payload as create (SQLite UNIQUE, etc.).
func (d *Driver) execPivotInsertIgnoreDuplicate(ctx context.Context, tx bun.IDB, pivotTable string, row map[string]interface{}, narrative string) error {
	if len(row) == 0 {
		return errors.New("empty pivot row")
	}
	keys := make([]string, 0, len(row))
	for k := range row {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	args := make([]interface{}, len(keys))
	qcols := make([]string, len(keys))
	eng := strings.ToLower(strings.TrimSpace(d.DriverCredential.Engine))
	for i, k := range keys {
		args[i] = row[k]
		switch eng {
		case _const.PostgreSQLDriver:
			qcols[i] = QuotePGIdent(k)
		default:
			qcols[i] = "`" + strings.ReplaceAll(k, "`", "``") + "`"
		}
	}
	ph := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	colList := strings.Join(qcols, ", ")
	var tblQ string
	switch eng {
	case _const.PostgreSQLDriver:
		tblQ = QuotePGIdent(pivotTable)
	default:
		tblQ = "`" + strings.ReplaceAll(pivotTable, "`", "``") + "`"
	}

	var sqlStr string
	switch eng {
	case _const.SQLiteDriver:
		sqlStr = fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", tblQ, colList, ph)
	case _const.MySQLDriver, _const.MariaDBDriver:
		sqlStr = fmt.Sprintf("INSERT IGNORE INTO %s (%s) VALUES (%s)", tblQ, colList, ph)
	case _const.PostgreSQLDriver:
		if len(keys) != 2 {
			return fmt.Errorf("pivot insert expected 2 columns for ON CONFLICT, got %d", len(keys))
		}
		c1, c2 := QuotePGIdent(keys[0]), QuotePGIdent(keys[1])
		sqlStr = fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT (%s, %s) DO NOTHING",
			tblQ, colList, ph, c1, c2)
	default:
		sqlStr = fmt.Sprintf("INSERT OR IGNORE INTO %s (%s) VALUES (%s)", tblQ, colList, ph)
	}
	d.logConnectBuilderSQL(narrative, "pivot insert (many-to-many link row)", sqlStr, args)
	_, err := tx.ExecContext(ctx, sqlStr, args...)
	return err
}

func connectDisconnectPivotTableName(cdp *models.ConnectDisconnectParam) string {
	if cdp == nil || cdp.ForwardConnectionType == nil || cdp.BackwardConnectionType == nil {
		return ""
	}
	if cdp.ForwardConnectionModelType != nil && cdp.BackwardConnectionModelType != nil {
		return relationPivotTableNameParts(
			cdp.ForwardConnectionModelType.Name,
			cdp.BackwardConnectionModelType.Name,
			relationKnownAs(cdp.ForwardConnectionType, cdp.BackwardConnectionType),
		)
	}
	return relationPivotTableName(cdp.ForwardConnectionType, cdp.BackwardConnectionType)
}

// connectDisconnectIsOneToOne is true for has_one↔has_one (symmetric FK columns on both model tables).
func connectDisconnectIsOneToOne(cdp *models.ConnectDisconnectParam) bool {
	if cdp == nil || cdp.ForwardConnectionType == nil || cdp.BackwardConnectionType == nil {
		return false
	}
	return cdp.ForwardConnectionType.Relation == "has_one" && cdp.BackwardConnectionType.Relation == "has_one"
}

// runDualHasOneConnectTx sets both FK columns for 1:1 inside an existing transaction.
func (d *Driver) runDualHasOneConnectTx(ctx context.Context, tx bun.IDB, root *models.CommonSystemParams, cdp *models.ConnectDisconnectParam, id string, narrative string) error {
	peerTbl := utility.PhysicalSQLTableName(cdp.ForwardConnectionType.Model)
	holderTbl := utility.PhysicalSQLTableName(cdp.BackwardConnectionType.Model)
	colOnPeerToHolder := relationFKColumnNameForModel(cdp.BackwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType)
	colOnHolderToPeer := relationFKColumnNameForModel(cdp.ForwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType)
	u1 := map[string]interface{}{colOnPeerToHolder: cdp.ForwardConnectionID}
	q1 := tx.NewUpdate().Table(peerTbl).Where("id = ?", id)
	q1 = applyBunHookWheresUpdate(d.Conf, ctx, root, q1)
	q1 = q1.Model(&u1)
	step1 := fmt.Sprintf("dual has_one 1/2: set FK on %s row id=%s (point toward %s id=%s)",
		humanizeSQLModelName(cdp.ForwardConnectionType.Model), id,
		humanizeSQLModelName(cdp.BackwardConnectionType.Model), cdp.ForwardConnectionID)
	d.logBunUpdateQuery(narrative, step1, q1)
	if _, err := q1.Exec(ctx); err != nil {
		return err
	}
	u2 := map[string]interface{}{colOnHolderToPeer: id}
	q2 := tx.NewUpdate().Table(holderTbl).Where("id = ?", cdp.ForwardConnectionID)
	q2 = applyBunHookWheresUpdate(d.Conf, ctx, root, q2)
	q2 = q2.Model(&u2)
	step2 := fmt.Sprintf("dual has_one 2/2: set FK on %s row id=%s (point back to %s id=%s)",
		humanizeSQLModelName(cdp.BackwardConnectionType.Model), cdp.ForwardConnectionID,
		humanizeSQLModelName(cdp.ForwardConnectionType.Model), id)
	d.logBunUpdateQuery(narrative, step2, q2)
	_, err := q2.Exec(ctx)
	return err
}

// runDualHasOneDisconnectTx clears both FK columns for 1:1 inside an existing transaction.
func (d *Driver) runDualHasOneDisconnectTx(ctx context.Context, tx bun.IDB, param *models.CommonSystemParams, cdp *models.ConnectDisconnectParam, id string) error {
	peerTbl := utility.PhysicalSQLTableName(cdp.ForwardConnectionType.Model)
	holderTbl := utility.PhysicalSQLTableName(cdp.BackwardConnectionType.Model)
	fkOnPeer := relationFKColumnNameForModel(cdp.BackwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType)
	fkOnHolder := relationFKColumnNameForModel(cdp.ForwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType)
	qu1 := tx.NewUpdate().Table(peerTbl).
		Set("? = NULL", bun.Ident(fkOnPeer)).
		Where("id = ?", id).
		Where("? = ?", bun.Ident(fkOnPeer), cdp.ForwardConnectionID)
	qu1 = applyBunHookWheresUpdate(d.Conf, ctx, param, qu1)
	if _, err := qu1.Exec(ctx); err != nil {
		return err
	}
	qu2 := tx.NewUpdate().Table(holderTbl).
		Set("? = NULL", bun.Ident(fkOnHolder)).
		Where("id = ?", cdp.ForwardConnectionID).
		Where("? = ?", bun.Ident(fkOnHolder), id)
	qu2 = applyBunHookWheresUpdate(d.Conf, ctx, param, qu2)
	_, err := qu2.Exec(ctx)
	return err
}

func (d *Driver) ConnectBuilder(ctx context.Context, root *models.CommonSystemParams) error {
	if root == nil || len(root.ConDisParam) == 0 {
		return nil
	}
	if locked := d.lockSQLiteWrite(); locked {
		defer d.unlockSQLiteWrite()
	}
	return d.runInTxOrBypass(ctx, func(ctx context.Context, tx bun.IDB) error {
		for _, cdp := range root.ConDisParam {
			for _, id := range cdp.ActionIDs {
				nar := connectBuilderConnectNarrative(cdp, id)
				logConnectBuilderBegin(nar)
				switch cdp.ConnectionType {
				case "forward":
					switch cdp.BackwardConnectionType.Relation {
					case "has_one":
						tableName := utility.PhysicalSQLTableName(cdp.ForwardConnectionType.Model)
						var err error
						if connectDisconnectIsOneToOne(cdp) {
							err = d.runDualHasOneConnectTx(ctx, tx, root, cdp, id, nar)
						} else {
							u := map[string]interface{}{
								relationFKColumnNameForModel(cdp.BackwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType): cdp.ForwardConnectionID,
							}
							qu := tx.NewUpdate().Table(tableName).Where("id = ?", id)
							qu = applyBunHookWheresUpdate(d.Conf, ctx, root, qu)
							qu = qu.Model(&u)
							step := fmt.Sprintf("forward has_one: set FK on %s row action_id=%s → points to %s forward_connection_id=%s",
								humanizeSQLModelName(cdp.ForwardConnectionType.Model), id,
								humanizeSQLModelName(cdp.BackwardConnectionType.Model), cdp.ForwardConnectionID)
							d.logBunUpdateQuery(nar, step, qu)
							_, err = qu.Exec(ctx)
						}
						if err != nil {
							return err
						}
					case "has_many":
						if cdp.ForwardConnectionType.Relation == "has_one" {
							tbl := utility.PhysicalSQLTableName(cdp.BackwardConnectionType.Model)
							fkCol := relationFKColumnNameForModel(cdp.ForwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType)
							u := map[string]interface{}{fkCol: id}
							qu := tx.NewUpdate().Table(tbl).Where("id = ?", cdp.ForwardConnectionID)
							qu = applyBunHookWheresUpdate(d.Conf, ctx, root, qu)
							qu = qu.Model(&u)
							step := fmt.Sprintf("forward has_many (parent has_one on %s): set FK on %s row id=%s → references %s id=%s (column %s)",
								humanizeSQLModelName(cdp.ForwardConnectionType.Model),
								humanizeSQLModelName(cdp.BackwardConnectionType.Model), cdp.ForwardConnectionID,
								humanizeSQLModelName(cdp.ForwardConnectionType.Model), id, fkCol)
							d.logBunUpdateQuery(nar, step, qu)
							if _, err := qu.Exec(ctx); err != nil {
								return err
							}
						} else {
							pivot := connectDisconnectPivotTableName(cdp)
							row := pivotManyManyInsertRow(cdp, id)
							if err := d.execPivotInsertIgnoreDuplicate(ctx, tx, pivot, row, nar); err != nil {
								return err
							}
						}
					}
				case "backward":
					switch cdp.ForwardConnectionType.Relation {
					case "has_one":
						tableName := utility.PhysicalSQLTableName(cdp.ForwardConnectionType.Model)
						var err error
						if connectDisconnectIsOneToOne(cdp) {
							err = d.runDualHasOneConnectTx(ctx, tx, root, cdp, id, nar)
						} else {
							u := map[string]interface{}{
								relationFKColumnNameForModel(cdp.BackwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType): cdp.ForwardConnectionID,
							}
							qu := tx.NewUpdate().Table(tableName).Where("id = ?", id)
							qu = applyBunHookWheresUpdate(d.Conf, ctx, root, qu)
							qu = qu.Model(&u)
							step := fmt.Sprintf("backward has_one: set FK on %s row action_id=%s → points to %s forward_connection_id=%s",
								humanizeSQLModelName(cdp.ForwardConnectionType.Model), id,
								humanizeSQLModelName(cdp.BackwardConnectionType.Model), cdp.ForwardConnectionID)
							d.logBunUpdateQuery(nar, step, qu)
							_, err = qu.Exec(ctx)
						}
						if err != nil {
							return err
						}
					case "has_many":
						if cdp.BackwardConnectionType.Relation == "has_one" {
							tbl := utility.PhysicalSQLTableName(cdp.ForwardConnectionType.Model)
							fkCol := relationFKColumnNameForModel(cdp.BackwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType)
							u := map[string]interface{}{fkCol: id}
							qu := tx.NewUpdate().Table(tbl).Where("id = ?", cdp.ForwardConnectionID)
							qu = applyBunHookWheresUpdate(d.Conf, ctx, root, qu)
							qu = qu.Model(&u)
							step := fmt.Sprintf("backward has_many (child has_one on %s): set FK on %s row id=%s → references %s id=%s (column %s)",
								humanizeSQLModelName(cdp.BackwardConnectionType.Model),
								humanizeSQLModelName(cdp.ForwardConnectionType.Model), cdp.ForwardConnectionID,
								humanizeSQLModelName(cdp.BackwardConnectionType.Model), id, fkCol)
							d.logBunUpdateQuery(nar, step, qu)
							if _, err := qu.Exec(ctx); err != nil {
								return err
							}
						} else {
							pivot := connectDisconnectPivotTableName(cdp)
							row := pivotManyManyInsertRow(cdp, id)
							if err := d.execPivotInsertIgnoreDuplicate(ctx, tx, pivot, row, nar); err != nil {
								return err
							}
						}
					}
				}
			}
		}
		return nil
	})
}

func (d *Driver) DisconnectBuilder(ctx context.Context, param *models.CommonSystemParams) error {
	if param == nil || len(param.ConDisParam) == 0 {
		return nil
	}
	if locked := d.lockSQLiteWrite(); locked {
		defer d.unlockSQLiteWrite()
	}
	return d.runInTxOrBypass(ctx, func(ctx context.Context, tx bun.IDB) error {
		for _, cdp := range param.ConDisParam {
			if cdp == nil || cdp.ForwardConnectionType == nil || cdp.BackwardConnectionType == nil {
				continue
			}
			for _, id := range cdp.ActionIDs {
				switch cdp.ConnectionType {
				case "forward":
					switch cdp.BackwardConnectionType.Relation {
					case "has_one":
						tableName := utility.PhysicalSQLTableName(cdp.ForwardConnectionType.Model)
						var err error
						if connectDisconnectIsOneToOne(cdp) {
							err = d.runDualHasOneDisconnectTx(ctx, tx, param, cdp, id)
						} else {
							fkCol := relationFKColumnNameForModel(cdp.BackwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType)
							qu := tx.NewUpdate().Table(tableName).
								Set("? = NULL", bun.Ident(fkCol)).
								Where("id = ?", id).
								Where("? = ?", bun.Ident(fkCol), cdp.ForwardConnectionID)
							qu = applyBunHookWheresUpdate(d.Conf, ctx, param, qu)
							_, err = qu.Exec(ctx)
						}
						if err != nil {
							return err
						}
					case "has_many":
						if cdp.ForwardConnectionType.Relation == "has_one" {
							tbl := utility.PhysicalSQLTableName(cdp.BackwardConnectionType.Model)
							fkCol := relationFKColumnNameForModel(cdp.ForwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType)
							qu := tx.NewUpdate().Table(tbl).
								Set("? = NULL", bun.Ident(fkCol)).
								Where("id = ?", cdp.ForwardConnectionID).
								Where("? = ?", bun.Ident(fkCol), id)
							qu = applyBunHookWheresUpdate(d.Conf, ctx, param, qu)
							if _, err := qu.Exec(ctx); err != nil {
								return err
							}
						} else {
							pivotTable := connectDisconnectPivotTableName(cdp)
							row := pivotManyManyInsertRow(cdp, id)
							qd := tx.NewDelete().Table(pivotTable)
							for k, v := range row {
								qd = qd.Where("? = ?", bun.Ident(k), v)
							}
							if _, err := qd.Exec(ctx); err != nil {
								return err
							}
						}
					}
				case "backward":
					switch cdp.ForwardConnectionType.Relation {
					case "has_one":
						tableName := utility.PhysicalSQLTableName(cdp.ForwardConnectionType.Model)
						var err error
						if connectDisconnectIsOneToOne(cdp) {
							err = d.runDualHasOneDisconnectTx(ctx, tx, param, cdp, id)
						} else {
							fkCol := relationFKColumnNameForModel(cdp.BackwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType)
							qu := tx.NewUpdate().Table(tableName).
								Set("? = NULL", bun.Ident(fkCol)).
								Where("id = ?", id).
								Where("? = ?", bun.Ident(fkCol), cdp.ForwardConnectionID)
							qu = applyBunHookWheresUpdate(d.Conf, ctx, param, qu)
							_, err = qu.Exec(ctx)
						}
						if err != nil {
							return err
						}
					case "has_many":
						if cdp.BackwardConnectionType.Relation == "has_one" {
							tbl := utility.PhysicalSQLTableName(cdp.ForwardConnectionType.Model)
							fkCol := relationFKColumnNameForModel(cdp.BackwardConnectionType.Model, cdp.ForwardConnectionType, cdp.BackwardConnectionType)
							qu := tx.NewUpdate().Table(tbl).
								Set("? = NULL", bun.Ident(fkCol)).
								Where("id = ?", cdp.ForwardConnectionID).
								Where("? = ?", bun.Ident(fkCol), id)
							qu = applyBunHookWheresUpdate(d.Conf, ctx, param, qu)
							if _, err := qu.Exec(ctx); err != nil {
								return err
							}
						} else {
							pivotTable := connectDisconnectPivotTableName(cdp)
							row := pivotManyManyInsertRow(cdp, id)
							qd := tx.NewDelete().Table(pivotTable)
							for k, v := range row {
								qd = qd.Where("? = ?", bun.Ident(k), v)
							}
							if _, err := qd.Exec(ctx); err != nil {
								return err
							}
						}
					}
				}
			}
		}
		return nil
	})
}

func (d *Driver) CheckProjectExists(ctx context.Context, projectId string) (bool, error) {

	var result int64

	switch d.DriverCredential.Engine {
	case _const.MySQLDriver:
		count, err := d.ORM.NewSelect().Table("information_schema.SCHEMATA").
			Where("SCHEMA_NAME = ?", projectId).Count(ctx)
		if err != nil {
			return false, err
		}
		result = int64(count)
		if result == 1 {
			return true, nil
		}
	case _const.PostgreSQLDriver:
		count, err := d.ORM.NewSelect().Table("pg_database").
			Where("datname = ?", projectId).Count(ctx)
		if err != nil {
			return false, err
		}
		result = int64(count)
		if result == 1 {
			return true, nil
		}
	}
	return false, nil
}

/* deprecated
func (d *Driver) GetAllPreviewDocumentsByModel(param models.CommonSystemParams) ([]*models.PreviewMode, error) {
	query, err := RootResolverQueryBuilder(param, true)
	if err != nil {
		return nil, err
	}

	var results []map[string]interface{}
	err = d.ORM.NewRaw(*query).Scan(ctx, &results)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return []*models.PreviewMode{}, nil
		} else {
			return nil, err
		}
	}

	var docs []*models.PreviewMode
	for _, res := range results {
		doc := &models.PreviewMode{}
		if val, ok := res["id"].([]byte); ok {
			doc.Id = string(val)
		}
		if val, ok := res["title"].(string); ok {
			doc.Title = val
		}
		if val, ok := res["status"].(string); ok {
			doc.Status = val
		} else {
			doc.Status = "draft" // default
		}
		if val, ok := res["icon"].(string); ok {
			doc.Icon = val
		}
		// filter doc title
		title := strip.StripTags(doc.Title)
		if len(title) > 35 {
			doc.Title = title[0:35] + "..."
		} else {
			doc.Title = title
		}
		docs = append(docs, doc)
	}
	return docs, nil
}
*/

func (d *Driver) GetSingleProjectDocumentBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {
	doc, err := d.GetSingleProjectDocument(ctx, param)
	if err != nil {
		return nil, err
	}
	return json.Marshal(doc)
}

func (d *Driver) GetSingleProjectDocument(ctx context.Context, param *models.CommonSystemParams) (*types.DefaultDocumentStructure, error) {

	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	returnType := SelectBuilder("y", local, param.Model, false)

	tableName := utility.PhysicalSQLTableName(param.Model.Name)
	result := map[string]interface{}{}
	hookWhere, hookArgs, err := singleDocHookWhereSQLAndArgs(d.Conf, ctx, param)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT %s FROM `%s` AS x LEFT JOIN meta as y on y.doc_id = x.id WHERE x.id = ?%s", strings.Join(returnType, ", "), tableName, hookWhere)
	args := append([]interface{}{param.DocumentID}, hookArgs...)
	err = d.ORM.NewRaw(query, args...).Scan(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &types.DefaultDocumentStructure{}, nil
		} else {
			return nil, err
		}
	}

	classification := d.BuildFieldClassification(param.Model.Fields)

	doc, err := CommonDocTransformation(param.Model, local, result, classification)
	if err != nil {
		return nil, err
	}

	doc.Type = param.Model.Name
	return doc, nil
}

func (d *Driver) BuildFieldClassification(_fields []*models.FieldInfo) *FieldClassification {
	classification := FieldClassification{}
	for _, f := range _fields {
		if f.FieldType == _const.MultilineField {
			classification.MultilineFields = append(classification.MultilineFields, f.Identifier)
		} else if f.FieldType == _const.MediaField && f.Validation != nil && f.Validation.IsGallery {
			classification.GalleryField = append(classification.GalleryField, f.Identifier)
		} else if f.FieldType == _const.MediaField {
			classification.PictureField = append(classification.PictureField, f.Identifier)
		} else if f.FieldType == _const.ListField && f.Validation != nil && (len(f.Validation.FixedListElements) == 0 || f.Validation.IsMultiChoice) {
			classification.ListFields = append(classification.ListFields, f.Identifier)
		} else if f.FieldType == _const.RepeatedField {
			if len(classification.RepeatedFields) == 0 {
				classification.RepeatedFields = make(map[string][]*models.FieldInfo)
			}
			classification.RepeatedFields[f.Identifier] = append(classification.RepeatedFields[f.Identifier], f.SubFieldInfo...)
		} else if f.FieldType == _const.ObjectField {
			classification.ObjectField = append(classification.ObjectField, f.Identifier)
		} else if f.FieldType == _const.BooleanField {
			classification.BooleanFields = append(classification.BooleanFields, f.Identifier)
		} else if f.FieldType == _const.DateField {
			classification.DateFields = append(classification.DateFields, f.Identifier)
		}
	}
	return &classification
}

func (d *Driver) GetSingleRawDocumentFromProject(ctx context.Context, param *models.CommonSystemParams) (interface{}, error) {
	if param.SinglePageData {
		return &types.DefaultDocumentStructure{
			Key:  param.DocumentID,
			ID:   param.DocumentID,
			Type: param.Model.Name,
			Data: map[string]interface{}{},
			Meta: &types.MetaField{
				LastModifiedBy: &types.SystemUser{},
			},
		}, nil
	}

	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	returnType := `
	 x.*, y.created_at AS sys_created_at, y.updated_at AS sys_updated_at, 
		y.created_by AS sys_created_by, y.updated_by AS sys_updated_by, 
		y.status as sys_status
	`

	tableName := utility.PhysicalSQLTableName(param.Model.Name)
	result := map[string]interface{}{}
	hookWhere, hookArgs, err := singleDocHookWhereSQLAndArgs(d.Conf, ctx, param)
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT %s FROM `%s` AS x LEFT JOIN meta as y on y.doc_id = x.id WHERE x.id = ?%s", returnType, tableName, hookWhere)
	args := append([]interface{}{param.DocumentID}, hookArgs...)
	err = d.ORM.NewRaw(query, args...).Scan(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &types.DefaultDocumentStructure{}, nil
		} else {
			return nil, err
		}
	}

	classification := d.BuildFieldClassification(param.Model.Fields)

	doc, err := CommonDocTransformation(param.Model, local, result, classification)
	if err != nil {
		return nil, err
	}

	doc.Type = param.Model.Name
	return doc, nil
}

// GetAllRelationDocumentsOfSingleDocument retrieves all relation data of a single document by ID.
func (s *Driver) GetAllRelationDocumentsOfSingleDocument_AUTOGEN(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error) {
	var relations []interface{}

	// Get all table names to find relation tables
	var tables []string
	err := s.ORM.NewRaw("SHOW TABLES").Scan(ctx, &tables)
	if err != nil {
		return nil, err
	}

	// Look for relation tables and query them
	for _, table := range tables {
		if strings.Contains(table, "_") && !strings.Contains(table, "meta") && !strings.Contains(table, "media") {
			// This looks like a relation table
			parts := strings.Split(table, "_")
			if len(parts) == 2 {
				// Query for relations where this document is the source
				var results []map[string]interface{}
				query1 := fmt.Sprintf("SELECT * FROM `%s` WHERE %s_id = ?", table, parts[0])
				s.ORM.NewRaw(query1, from).Scan(ctx, &results)

				for _, result := range results {
					relations = append(relations, result)
				}

				// Query for relations where this document is the target
				query2 := fmt.Sprintf("SELECT * FROM `%s` WHERE %s_id = ?", table, parts[1])
				s.ORM.NewRaw(query2, from).Scan(ctx, &results)

				for _, result := range results {
					relations = append(relations, result)
				}
			}
		}
	}

	return relations, nil
}

func (d *Driver) GetAllRelationDocumentsOfSingleDocument(ctx context.Context, from string, arg *models.CommonSystemParams) (interface{}, error) {
	// query relations and find all docs
	arg.DocumentIDs = []string{arg.DocumentID}
	arg.OnlyReturnCount = true
	query, relArgs, relationType, err := BuildCombinedRelationQuery(d.Conf, "", from, arg)
	if err != nil {
		return nil, err
	}

	switch *relationType {
	case "has_many":
		var result []map[string]interface{}
		if len(relArgs) > 0 {
			err = d.ORM.NewRaw(query, relArgs...).Scan(ctx, &result)
		} else {
			err = d.ORM.NewRaw(query).Scan(ctx, &result)
		}
		if err != nil {
			return nil, err
		}
		var docs []*models.PreviewMode
		for _, res := range result {
			doc := models.PreviewMode{}
			if val, ok := res["id"].([]byte); ok {
				id := string(val)
				doc.ID = id
			} else {
				// if no id then return nil
				return []*models.PreviewMode{}, nil
			}
			if val, ok := res["title"].(string); ok {
				doc.Title = val
			}
			if val, ok := res["icon"].(string); ok {
				doc.Icon = val
			}
			if val, ok := res["status"].(string); ok {
				doc.Status = val
			}
			docs = append(docs, &doc)
		}
		return docs, nil
	case "has_one":
		result := map[string]interface{}{}
		if len(relArgs) > 0 {
			err = d.ORM.NewRaw(query, relArgs...).Scan(ctx, &result)
		} else {
			err = d.ORM.NewRaw(query).Scan(ctx, &result)
		}
		if err != nil {
			return nil, err
		}
		doc := models.PreviewMode{}
		if val, ok := result["id"].([]byte); ok {
			id := string(val)
			doc.ID = id
		} else {
			// if no id then return nil
			return nil, nil
		}
		if val, ok := result["title"].(string); ok {
			doc.Title = val
		}
		if val, ok := result["icon"].(string); ok {
			doc.Icon = val
		}
		if val, ok := result["status"].(string); ok {
			doc.Status = val
		}
		return doc, nil
	}

	return nil, errors.New("Invalid Relation Type")
}

func (d *Driver) CountMedias(ctx context.Context, projectId string, param *graphql.ResolveParams) (int, error) {
	return 0, nil
}

func (d *Driver) ListMedias(ctx context.Context, projectId string, param *graphql.ResolveParams) ([]*models.FileDetails, error) {
	query, err := RootResolverMediaQueryBuilder(param)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	err = d.ORM.NewRaw(query).Scan(ctx, &result)
	if err != nil {
		return nil, err
	}

	var docs []*models.FileDetails
	for _, res := range result {
		doc, err := MediaDocTransformation("media", res)
		if err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}

	return docs, nil
}

func (f *Driver) CountMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams, previewMode bool) (int, error) {

	if err := f.maybeEnsureRelationDDL(ctx, param); err != nil {
		return 0, err
	}
	query, args, err := RootResolverQueryBuilder(f.Conf, param, true)
	if err != nil {
		return 0, err
	}

	var result int64
	if len(args) > 0 {
		err = f.ORM.NewRaw(query, args...).Scan(ctx, &result)
	} else {
		err = f.ORM.NewRaw(query).Scan(ctx, &result)
	}
	if err != nil {
		return 0, err
	}

	return int(result), nil

}

func (d *Driver) QueryMultiDocumentOfProjectBytes(ctx context.Context, param *models.CommonSystemParams) ([]byte, error) {

	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	if err := d.maybeEnsureRelationDDL(ctx, param); err != nil {
		return nil, err
	}
	query, args, err := RootResolverQueryBuilder(d.Conf, param, false)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if len(args) > 0 {
		err = d.ORM.NewRaw(query, args...).Scan(ctx, &result)
	} else {
		err = d.ORM.NewRaw(query).Scan(ctx, &result)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { // send empty response for table
			return []byte{}, nil
		}
		return nil, err
	}

	classification := d.BuildFieldClassification(param.Model.Fields)

	var docs []*types.DefaultDocumentStructure
	for _, res := range result {
		doc, err := CommonDocTransformation(param.Model, local, res, classification)
		if err != nil {
			return nil, err
		}
		doc.Type = param.Model.Name
		docs = append(docs, doc)
	}

	return []byte{}, nil
}

func (d *Driver) QueryMultiDocumentOfProject(ctx context.Context, param *models.CommonSystemParams) ([]*types.DefaultDocumentStructure, error) {

	if err := d.maybeEnsureRelationDDL(ctx, param); err != nil {
		return nil, err
	}
	query, args, err := RootResolverQueryBuilder(d.Conf, param, false)
	if err != nil {
		return nil, err
	}

	var result []map[string]interface{}
	if len(args) > 0 {
		err = d.ORM.NewRaw(query, args...).Scan(ctx, &result)
	} else {
		err = d.ORM.NewRaw(query).Scan(ctx, &result)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) { // send empty response for table
			return []*types.DefaultDocumentStructure{}, nil
		}
		return nil, err
	}

	classification := d.BuildFieldClassification(param.Model.Fields)
	var local string
	if val, ok := param.ResolveParams.Args["local"].(string); ok {
		local = val
	}

	var docs []*types.DefaultDocumentStructure
	for _, res := range result {
		doc, err := CommonDocTransformation(param.Model, local, res, classification)
		if err != nil {
			return nil, err
		}
		doc.Type = param.Model.Name
		docs = append(docs, doc)
	}

	return docs, nil
}

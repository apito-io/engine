package sqlite

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/uptrace/bun"
)

type pragmaTableCol struct {
	CID       int         `bun:"cid"`
	Name      string      `bun:"name"`
	Type      string      `bun:"type"`
	Notnull   int         `bun:"notnull"`
	DfltValue interface{} `bun:"dflt_value"`
	PK        int         `bun:"pk"`
}

type pragmaFKRow struct {
	ID       int    `bun:"id"`
	Seq      int    `bun:"seq"`
	Table    string `bun:"table"`
	From     string `bun:"from"`
	To       string `bun:"to"`
	OnUpdate string `bun:"on_update"`
	OnDelete string `bun:"on_delete"`
	Match    string `bun:"match"`
}

type savedIndex struct {
	CreateSQL string
}

func sqliteQuoteIdent(s string) string {
	s = strings.TrimSpace(s)
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func groupForeignKeyRows(rows []pragmaFKRow) [][]pragmaFKRow {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ID != rows[j].ID {
			return rows[i].ID < rows[j].ID
		}
		return rows[i].Seq < rows[j].Seq
	})
	var out [][]pragmaFKRow
	var cur []pragmaFKRow
	curID := -1
	for _, r := range rows {
		if r.ID != curID {
			if len(cur) > 0 {
				out = append(out, cur)
			}
			cur = nil
			curID = r.ID
		}
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

func filterFKGroups(groups [][]pragmaFKRow, dropCol string) [][]pragmaFKRow {
	var out [][]pragmaFKRow
Groups:
	for _, g := range groups {
		for _, r := range g {
			if strings.EqualFold(strings.TrimSpace(r.From), dropCol) {
				continue Groups
			}
		}
		out = append(out, g)
	}
	return out
}

func buildSQLiteFKClause(group []pragmaFKRow) string {
	if len(group) == 0 {
		return ""
	}
	sort.Slice(group, func(i, j int) bool { return group[i].Seq < group[j].Seq })
	refTable := strings.TrimSpace(group[0].Table)
	if refTable == "" {
		return ""
	}
	rt := sqliteQuoteIdent(refTable)

	var fromCols []string
	var toCols []string
	hasExplicitTo := false
	for _, r := range group {
		fromCols = append(fromCols, sqliteQuoteIdent(strings.TrimSpace(r.From)))
		t := strings.TrimSpace(r.To)
		toCols = append(toCols, t)
		if t != "" {
			hasExplicitTo = true
		}
	}

	var refSuffix string
	switch {
	case len(fromCols) == 1 && !hasExplicitTo:
		refSuffix = rt
	case len(fromCols) == 1 && hasExplicitTo:
		refSuffix = fmt.Sprintf("%s (%s)", rt, sqliteQuoteIdent(toCols[0]))
	case len(fromCols) > 1 && hasExplicitTo:
		var pq []string
		for _, t := range toCols {
			if t == "" {
				return ""
			}
			pq = append(pq, sqliteQuoteIdent(t))
		}
		refSuffix = fmt.Sprintf("%s (%s)", rt, strings.Join(pq, ", "))
	case len(fromCols) > 1 && !hasExplicitTo:
		refSuffix = rt
	default:
		return ""
	}

	s := fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s", strings.Join(fromCols, ", "), refSuffix)

	onDel := strings.TrimSpace(strings.ToUpper(group[0].OnDelete))
	if onDel != "" && onDel != "NO ACTION" {
		s += " ON DELETE " + onDel
	}
	onUp := strings.TrimSpace(strings.ToUpper(group[0].OnUpdate))
	if onUp != "" && onUp != "NO ACTION" {
		s += " ON UPDATE " + onUp
	}
	return s
}

// collectSurvivingIndexes reads all user-defined indexes from sqlite_master, filters out
// any that reference the dropped column, and returns CREATE INDEX SQL to replay after rebuild.
func collectSurvivingIndexes(ctx context.Context, db bun.IDB, physicalTable, dropCol string) ([]savedIndex, error) {
	type indexMaster struct {
		Name string  `bun:"name"`
		SQL  *string `bun:"sql"`
	}
	var indexes []indexMaster
	if err := db.NewRaw(
		`SELECT name, sql FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND sql IS NOT NULL`,
		physicalTable,
	).Scan(ctx, &indexes); err != nil {
		return nil, err
	}

	var out []savedIndex
	for _, idx := range indexes {
		if idx.SQL == nil || strings.TrimSpace(*idx.SQL) == "" {
			continue
		}
		type indexCol struct {
			Name string `bun:"name"`
		}
		var cols []indexCol
		if err := db.NewRaw(`SELECT name FROM pragma_index_info(?)`, idx.Name).Scan(ctx, &cols); err != nil {
			continue
		}
		skip := false
		for _, c := range cols {
			if strings.EqualFold(strings.TrimSpace(c.Name), dropCol) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		out = append(out, savedIndex{CreateSQL: *idx.SQL})
	}
	return out, nil
}

func patchSQLiteTableCreateSQL(ctx context.Context, db bun.IDB, physicalTable, createSQL string) error {
	if _, err := db.NewRaw(`PRAGMA writable_schema = ON`).Exec(ctx); err != nil {
		return err
	}
	var schemaVersion int
	_ = db.NewRaw(`PRAGMA schema_version`).Scan(ctx, &schemaVersion)
	if _, err := db.NewRaw(`UPDATE sqlite_schema SET sql = ? WHERE type = 'table' AND name = ?`, createSQL, physicalTable).Exec(ctx); err != nil {
		_, _ = db.NewRaw(`PRAGMA writable_schema = OFF`).Exec(ctx)
		return err
	}
	if schemaVersion > 0 {
		_, _ = db.NewRaw(fmt.Sprintf(`PRAGMA schema_version = %d`, schemaVersion+1)).Exec(ctx)
	}
	_, err := db.NewRaw(`PRAGMA writable_schema = OFF`).Exec(ctx)
	return err
}

// sqliteRebuildTableWithoutColumnTx recreates a table without one column (SQLite-like).
// Preserves surviving FOREIGN KEY constraints (tested individually) and all indexes that
// don't reference the dropped column.
func sqliteRebuildTableWithoutColumnTx(ctx context.Context, db bun.IDB, physicalTable, dropCol string) error {
	dropCol = strings.TrimSpace(dropCol)
	if physicalTable == "" || dropCol == "" {
		return fmt.Errorf("sqliteRebuildTableWithoutColumnTx: empty table or column")
	}
	var fkPragma int
	_ = db.NewRaw(`PRAGMA foreign_keys`).Scan(ctx, &fkPragma)

	// ── 1. Collect FK metadata ─────────────────────────────────────────────
	var fkRows []pragmaFKRow
	err := db.NewRaw(`
SELECT id, seq, "table", "from", "to",
  COALESCE(on_update, '') AS on_update,
  COALESCE(on_delete, '') AS on_delete,
  COALESCE(match, '') AS match
FROM pragma_foreign_key_list(?)`, physicalTable).Scan(ctx, &fkRows)
	if err != nil {
		return err
	}
	fkGroups := filterFKGroups(groupForeignKeyRows(fkRows), dropCol)
	var fkClauses []string
	for _, g := range fkGroups {
		if cl := buildSQLiteFKClause(g); cl != "" {
			fkClauses = append(fkClauses, cl)
		}
	}
	// ── 2. Collect surviving indexes ───────────────────────────────────────
	survivingIndexes, idxErr := collectSurvivingIndexes(ctx, db, physicalTable, dropCol)
	if idxErr != nil {
		return idxErr
	}

	// ── 3. Build column definitions ────────────────────────────────────────
	var cols []pragmaTableCol
	if err := db.NewRaw(`PRAGMA table_info(?)`, physicalTable).Scan(ctx, &cols); err != nil {
		return err
	}

	tmp := physicalTable + "__apito_drop_rel_tmp"
	tq := strings.ReplaceAll(tmp, "`", "``")
	pt := strings.ReplaceAll(physicalTable, "`", "``")

	var pkCount int
	var solePKCol string
	for _, c := range cols {
		if strings.EqualFold(strings.TrimSpace(c.Name), dropCol) {
			continue
		}
		if c.PK > 0 {
			pkCount++
			if solePKCol == "" {
				solePKCol = strings.TrimSpace(c.Name)
			}
		}
	}

	var colDefs []string
	for _, c := range cols {
		n := strings.TrimSpace(c.Name)
		if strings.EqualFold(n, dropCol) {
			continue
		}
		typ := strings.TrimSpace(c.Type)
		if typ == "" {
			typ = "TEXT"
		}
		line := fmt.Sprintf("`%s` %s", strings.ReplaceAll(n, "`", "``"), typ)
		if c.Notnull != 0 {
			line += " NOT NULL"
		}
		if pkCount == 1 && strings.EqualFold(n, solePKCol) {
			line += " PRIMARY KEY"
		}
		colDefs = append(colDefs, line)
	}
	if len(colDefs) == 0 {
		return fmt.Errorf("sqliteRebuildTableWithoutColumnTx: no columns left after drop")
	}

	// ── 4. Keep all surviving FK clauses ──────────────────────────────────
	// The caller runs this SQLite rebuild outside RunInTx with PRAGMA foreign_keys=OFF, so even FKs
	// pointing at parent tables with damaged PK metadata can be copied as schema metadata.
	validFKClauses := fkClauses

	// ── 5. INSERT SQL (used by both attempts) ──────────────────────────────
	var selCols []string
	for _, c := range cols {
		n := strings.TrimSpace(c.Name)
		if strings.EqualFold(n, dropCol) {
			continue
		}
		selCols = append(selCols, fmt.Sprintf("`%s`", strings.ReplaceAll(n, "`", "``")))
	}
	insertSQL := fmt.Sprintf("INSERT INTO `%s` (%s) SELECT %s FROM `%s`",
		tq, strings.Join(selCols, ", "), strings.Join(selCols, ", "), pt)

	// ── 6. CREATE temp + INSERT ────────────────────────────────────────────
	allParts := colDefs
	if len(validFKClauses) > 0 {
		allParts = append(append([]string{}, colDefs...), validFKClauses...)
	}
	preservedCreateSQL := fmt.Sprintf("CREATE TABLE `%s` (\n%s\n)", pt, strings.Join(allParts, ",\n"))
	tempCreateSQL := fmt.Sprintf("CREATE TABLE `%s` (\n%s\n)", tq, strings.Join(allParts, ",\n"))
	createSQL := tempCreateSQL
	patchSchemaAfterRename := false
	if fkPragma != 0 && len(validFKClauses) > 0 {
		// Remote libsql/Turso may keep foreign_keys=ON even after PRAGMA foreign_keys=OFF. In that
		// state INSERT into a temp table with FK clauses fails with foreign key mismatch if a parent
		// table has damaged PK metadata. Copy rows through bare columns, then restore FK metadata.
		createSQL = fmt.Sprintf("CREATE TABLE `%s` (\n%s\n)", tq, strings.Join(colDefs, ",\n"))
		patchSchemaAfterRename = true
	}
	_, _ = db.NewRaw(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", tq)).Exec(ctx)
	if _, err := db.NewRaw(createSQL).Exec(ctx); err != nil {
		return fmt.Errorf("sqlite rebuild create temp: %w", err)
	}
	if _, err := db.NewRaw(insertSQL).Exec(ctx); err != nil {
		return fmt.Errorf("sqlite rebuild insert: %w", err)
	}

	// ── 7. Swap tables ────────────────────────────────────────────────────
	if _, err := db.NewRaw(fmt.Sprintf("DROP TABLE `%s`", pt)).Exec(ctx); err != nil {
		return err
	}
	if _, err := db.NewRaw(fmt.Sprintf("ALTER TABLE `%s` RENAME TO `%s`", tq, pt)).Exec(ctx); err != nil {
		return err
	}

	if patchSchemaAfterRename {
		if err := patchSQLiteTableCreateSQL(ctx, db, physicalTable, preservedCreateSQL); err != nil {
			return fmt.Errorf("sqlite rebuild patch schema: %w", err)
		}
	}

	// ── 8. Recreate surviving indexes ──────────────────────────────────────
	for _, si := range survivingIndexes {
		if _, err := db.NewRaw(si.CreateSQL).Exec(ctx); err != nil {
			return fmt.Errorf("sqlite rebuild recreate index: %w", err)
		}
	}
	return nil
}

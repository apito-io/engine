package sql

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/apito-io/engine/utility"
)

const envTursoCounterTriggers = "TURSO_ENABLE_COUNTER_TRIGGERS"

func tursoCounterTriggersEnabled() bool {
	return strings.EqualFold(strings.TrimSpace(os.Getenv(envTursoCounterTriggers)), "true")
}

func (S *SQLDriver) ensureApitoRowCountsTable(ctx context.Context) error {
	if _, err := S.ORM.NewRaw(`
		CREATE TABLE IF NOT EXISTS _apito_row_counts (
			table_name TEXT NOT NULL,
			tenant_id TEXT NOT NULL DEFAULT '',
			row_count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (table_name, tenant_id)
		);`).Exec(ctx); err != nil {
		return err
	}
	return nil
}

func (S *SQLDriver) modelTableHasTenantIDColumn(ctx context.Context, table string) (bool, error) {
	var n int
	err := S.ORM.NewRaw(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = 'tenant_id'`, table).Scan(ctx, &n)
	return n > 0, err
}

// installRowCountTriggersForModel creates INSERT/DELETE triggers to maintain _apito_row_counts (SQLite-like only).
func (S *SQLDriver) installRowCountTriggersForModel(ctx context.Context, model *models.ModelType) error {
	if S == nil || model == nil || !tursoCounterTriggersEnabled() || !engineIsSQLiteLike(S.DriverCredential.Engine) {
		return nil
	}
	if err := S.ensureApitoRowCountsTable(ctx); err != nil {
		return err
	}
	tbl := utility.SingularResourceName(model.Name)
	qtbl := strings.ReplaceAll(tbl, "`", "``")
	tlit := strings.ReplaceAll(tbl, "'", "''")
	hasTenant, err := S.modelTableHasTenantIDColumn(ctx, tbl)
	if err != nil {
		return err
	}
	tenantExpr := `''`
	if hasTenant {
		tenantExpr = `COALESCE(NEW.tenant_id, '')`
	}
	tenantExprOld := `''`
	if hasTenant {
		tenantExprOld = `COALESCE(OLD.tenant_id, '')`
	}

	trAI := fmt.Sprintf("tr_rc_%s_ai", strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, tbl))
	trAD := fmt.Sprintf("tr_rc_%s_ad", strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, tbl))

	drop := fmt.Sprintf("DROP TRIGGER IF EXISTS `%s`; DROP TRIGGER IF EXISTS `%s`;", trAI, trAD)
	if _, err := S.ORM.NewRaw(drop).Exec(ctx); err != nil {
		return err
	}

	ai := fmt.Sprintf(`
CREATE TRIGGER `+"`%s`"+` AFTER INSERT ON `+"`%s`"+` BEGIN
  INSERT INTO _apito_row_counts(table_name, tenant_id, row_count)
  VALUES('%s', %s, 1)
  ON CONFLICT(table_name, tenant_id) DO UPDATE SET row_count = row_count + 1;
END;`, trAI, qtbl, tlit, tenantExpr)

	ad := fmt.Sprintf(`
CREATE TRIGGER `+"`%s`"+` AFTER DELETE ON `+"`%s`"+` BEGIN
  UPDATE _apito_row_counts SET row_count = row_count - 1
  WHERE table_name = '%s' AND tenant_id = %s;
END;`, trAD, qtbl, tlit, tenantExprOld)

	if _, err := S.ORM.NewRaw(ai).Exec(ctx); err != nil {
		return err
	}
	if _, err := S.ORM.NewRaw(ad).Exec(ctx); err != nil {
		return err
	}

	// Seed current row total (no per-tenant split unless tenant_id exists and caller backfills separately).
	var n int64
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM `%s`", qtbl)
	if err := S.ORM.NewRaw(countQ).Scan(ctx, &n); err != nil {
		return err
	}
	_, err = S.ORM.NewRaw(`
		INSERT INTO _apito_row_counts(table_name, tenant_id, row_count) VALUES (?, '', ?)
		ON CONFLICT(table_name, tenant_id) DO UPDATE SET row_count = excluded.row_count`,
		tbl, n).Exec(ctx)
	return err
}

// tenantIDEQFromWhere reports (value, true) when Args["where"] is exactly { tenant_id: { eq: "<non-empty>" } }.
func tenantIDEQFromWhere(args map[string]interface{}) (string, bool) {
	if args == nil {
		return "", false
	}
	w, ok := args["where"].(map[string]interface{})
	if !ok || len(w) != 1 {
		return "", false
	}
	tm, ok := w["tenant_id"].(map[string]interface{})
	if !ok || len(tm) != 1 {
		return "", false
	}
	v, ok := tm["eq"]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	return s, true
}

// tryCountFromRowCountTable returns (count, true, nil) when the fast path applies.
func (S *SQLDriver) tryCountFromRowCountTable(ctx context.Context, param *models.CommonSystemParams) (int64, bool, error) {
	if !tursoCounterTriggersEnabled() || !engineIsSQLiteLike(S.DriverCredential.Engine) {
		return 0, false, nil
	}
	if param == nil || param.Model == nil || param.ResolveParams == nil {
		return 0, false, nil
	}
	tenantWhere, whereOnlyTenant := tenantIDEQFromWhere(param.ResolveParams.Args)
	if w, ok := param.ResolveParams.Args["where"].(map[string]interface{}); ok && len(w) > 0 && !whereOnlyTenant {
		return 0, false, nil
	}
	if _, ok := param.ResolveParams.Args["connection"].(map[string]interface{}); ok {
		return 0, false, nil
	}
	modelName := utility.SingularResourceName(param.Model.Name)
	if permission, ok := utility.LookupAPIPermission(param.Role, modelName); ok && permission.Read == "own" {
		return 0, false, nil
	}
	if err := S.ensureApitoRowCountsTable(ctx); err != nil {
		return 0, false, err
	}
	tenantID := ""
	cfg := effectiveCfg(S.Conf, param)
	if cfg != nil && cfg.QueryFilterHook != nil {
		for _, f := range cfg.QueryFilterHook(hookCtx(param), param) {
			if f == nil {
				continue
			}
			vn := strings.TrimSpace(f.Variable)
			if vn != "" && vn != "x" {
				return 0, false, nil
			}
			if strings.TrimSpace(f.Key) != "tenant_id" {
				return 0, false, nil
			}
			if v, ok := f.Value.(string); ok {
				if tenantID != "" && tenantID != v {
					return 0, false, nil
				}
				tenantID = v
			} else {
				return 0, false, nil
			}
		}
	}
	if whereOnlyTenant {
		if tenantID != "" && tenantID != tenantWhere {
			return 0, false, nil
		}
		tenantID = tenantWhere
	}
	hasTenant, err := S.modelTableHasTenantIDColumn(ctx, utility.SingularResourceName(param.Model.Name))
	if err != nil {
		return 0, false, err
	}
	if whereOnlyTenant && !hasTenant {
		return 0, false, nil
	}
	if hasTenant && tenantID == "" {
		return 0, false, nil
	}
	if !hasTenant {
		tenantID = ""
	}
	var n int64
	q := `SELECT row_count FROM _apito_row_counts WHERE table_name = ? AND tenant_id = ?`
	if err := S.ORM.NewRaw(q, utility.SingularResourceName(param.Model.Name), tenantID).Scan(ctx, &n); err != nil {
		return 0, false, nil
	}
	return n, true, nil
}

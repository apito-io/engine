package sql

import (
	"context"
	"strings"

	_const "github.com/apito-io/engine/const"
)

// ensureSystemSecondaryIndexes creates idempotent secondary indexes for hot system queries.
func (p *SystemSQLDriver) ensureSystemSecondaryIndexes(ctx context.Context) error {
	if p == nil || p.ORM == nil || p.DriverCredential == nil {
		return nil
	}
	eng := strings.ToLower(strings.TrimSpace(p.DriverCredential.Engine))
	var stmts []string
	switch eng {
	case strings.ToLower(_const.PostgreSQLDriver):
		stmts = []string{
			`CREATE INDEX IF NOT EXISTS idx_audit_logs_project_created ON audit_logs (project_id, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created ON audit_logs (user_id, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_webhooks_project_id ON webhooks (project_id)`,
			`CREATE INDEX IF NOT EXISTS idx_project_tokens_token ON project_tokens (token)`,
		}
	case strings.ToLower(_const.MySQLDriver), strings.ToLower(_const.MariaDBDriver):
		stmts = []string{
			`CREATE INDEX IF NOT EXISTS idx_audit_logs_project_created ON audit_logs (project_id, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created ON audit_logs (user_id, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_webhooks_project_id ON webhooks (project_id)`,
			`CREATE INDEX IF NOT EXISTS idx_project_tokens_token ON project_tokens (token)`,
		}
	case strings.ToLower(_const.SQLiteDriver):
		stmts = []string{
			`CREATE INDEX IF NOT EXISTS idx_audit_logs_project_created ON audit_logs (project_id, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created ON audit_logs (user_id, created_at DESC)`,
			`CREATE INDEX IF NOT EXISTS idx_webhooks_project_id ON webhooks (project_id)`,
			`CREATE INDEX IF NOT EXISTS idx_project_tokens_token ON project_tokens (token)`,
		}
	default:
		return nil
	}
	for _, q := range stmts {
		if _, err := p.ORM.NewRaw(q).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

package sqlite

import (
	"context"
	"fmt"

	"github.com/apito-io/engine/models"
)

func (d *Driver) ensureSystemSecondaryIndexes(ctx context.Context) error {
	if d == nil || d.ORM == nil {
		return nil
	}
	projectUsers := models.ProjectUsersTableName
	stmts := []string{
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_project_created ON audit_logs (project_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_created ON audit_logs (user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_webhooks_project_id ON webhooks (project_id)`,
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_users_project_created ON %s (project_id, created_at DESC)`, projectUsers),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_users_project_username ON %s (project_id, username)`, projectUsers),
		fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_users_project_email ON %s (project_id, email)`, projectUsers),
		`CREATE INDEX IF NOT EXISTS idx_project_tokens_token ON project_tokens (token)`,
	}
	for _, q := range stmts {
		if _, err := d.ORM.NewRaw(q).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

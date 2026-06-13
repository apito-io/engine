package sqlite

import (
	"github.com/apito-io/engine/database/system/sqlcommon"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const (
	colAuthenticationSettings = "authentication_settings"
	colStorageSettings          = "storage_settings"
	colLegacyAuthSettings       = "auth_settings"
)

func ensureProjectSettingsSQLColumns(ctx context.Context, db *bun.DB) error {
	if db == nil {
		return errors.New("sql: nil ORM for project settings migration")
	}
	switch db.Dialect().Name() {
	case dialect.SQLite:
		stmts := []string{
			`ALTER TABLE projects ADD COLUMN ` + colAuthenticationSettings + ` TEXT`,
			`ALTER TABLE projects ADD COLUMN ` + colStorageSettings + ` TEXT`,
		}
		for _, q := range stmts {
			if err := sqlcommon.ExecAlterIgnoreDuplicate(ctx, db, q); err != nil {
				return err
			}
		}
	case dialect.PG:
		stmts := []string{
			`ALTER TABLE projects ADD COLUMN IF NOT EXISTS ` + colAuthenticationSettings + ` JSONB`,
			`ALTER TABLE projects ADD COLUMN IF NOT EXISTS ` + colStorageSettings + ` JSONB`,
		}
		for _, q := range stmts {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
	case dialect.MySQL:
		stmts := []string{
			`ALTER TABLE projects ADD COLUMN ` + colAuthenticationSettings + ` JSON NULL`,
			`ALTER TABLE projects ADD COLUMN ` + colStorageSettings + ` JSON NULL`,
		}
		for _, q := range stmts {
			if err := sqlcommon.ExecAlterIgnoreDuplicate(ctx, db, q); err != nil {
				return err
			}
		}
	default:
		return nil
	}
	return nil
}

func hydrateProjectSettingsFromDB(ctx context.Context, db *bun.DB, project *models.Project) error {
	if db == nil || project == nil || project.ID == "" {
		return nil
	}
	var authRaw, storageRaw, legacyAuth sql.NullString
	var err error
	if db.Dialect().Name() == dialect.PG {
		err = db.QueryRowContext(ctx, `
SELECT `+colAuthenticationSettings+`::text, `+colStorageSettings+`::text, `+colLegacyAuthSettings+`::text
FROM projects WHERE id = $1`, project.ID).Scan(&authRaw, &storageRaw, &legacyAuth)
	} else {
		err = db.QueryRowContext(ctx, `
SELECT `+colAuthenticationSettings+`, `+colStorageSettings+`, `+colLegacyAuthSettings+`
FROM projects WHERE id = ?`, project.ID).Scan(&authRaw, &storageRaw, &legacyAuth)
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if strings.Contains(strings.ToLower(err.Error()), "no such column") ||
			strings.Contains(strings.ToLower(err.Error()), "does not exist") {
			return nil
		}
		return err
	}
	applyAuthenticationSettingsJSON(project, authRaw)
	if project.AuthenticationSettings == nil {
		applyLegacyAuthSettingsJSON(project, legacyAuth)
	}
	applyStorageSettingsJSON(project, storageRaw)
	return nil
}

func applyAuthenticationSettingsJSON(project *models.Project, raw sql.NullString) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return
	}
	var as models.AuthenticationSettings
	if err := json.Unmarshal([]byte(raw.String), &as); err != nil {
		return
	}
	project.AuthenticationSettings = &as
}

func applyLegacyAuthSettingsJSON(project *models.Project, raw sql.NullString) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return
	}
	var as models.AuthenticationSettings
	if err := json.Unmarshal([]byte(raw.String), &as); err != nil {
		return
	}
	project.AuthenticationSettings = &as
}

func applyStorageSettingsJSON(project *models.Project, raw sql.NullString) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return
	}
	var st models.StorageSettings
	if err := json.Unmarshal([]byte(raw.String), &st); err != nil {
		return
	}
	project.StorageSettings = &st
}

func marshalSettingsColumn(v interface{}) (interface{}, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	if len(strings.TrimSpace(string(b))) == 0 || string(b) == "null" || string(b) == "{}" {
		return nil, nil
	}
	return string(b), nil
}

func (d *Driver) SaveProjectAuthenticationSettings(ctx context.Context, projectID string, auth *models.AuthenticationSettings) error {
	if d == nil || d.ORM == nil {
		return errors.New("sql: nil driver")
	}
	if strings.TrimSpace(projectID) == "" {
		return errors.New("project id required")
	}
	authJSON, err := marshalSettingsColumn(auth)
	if err != nil {
		return err
	}
	if d.ORM.Dialect().Name() == dialect.PG {
		_, err = d.ORM.ExecContext(ctx, `
UPDATE projects SET `+colAuthenticationSettings+` = $1::jsonb, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
			authJSON, projectID)
		return err
	}
	_, err = d.ORM.ExecContext(ctx, `
UPDATE projects SET `+colAuthenticationSettings+` = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		authJSON, projectID)
	return err
}

func (d *Driver) SaveProjectStorageSettings(ctx context.Context, projectID string, storage *models.StorageSettings) error {
	if d == nil || d.ORM == nil {
		return errors.New("sql: nil driver")
	}
	if strings.TrimSpace(projectID) == "" {
		return errors.New("project id required")
	}
	storageJSON, err := marshalSettingsColumn(storage)
	if err != nil {
		return err
	}
	if d.ORM.Dialect().Name() == dialect.PG {
		_, err = d.ORM.ExecContext(ctx, `
UPDATE projects SET `+colStorageSettings+` = $1::jsonb, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
			storageJSON, projectID)
		return err
	}
	_, err = d.ORM.ExecContext(ctx, `
UPDATE projects SET `+colStorageSettings+` = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		storageJSON, projectID)
	return err
}

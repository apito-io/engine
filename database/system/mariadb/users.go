package mariadb

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
)

func phoneOrLegacyUsernameMatch() string {
	return "(LOWER(TRIM(COALESCE(phone, ''))) = ? OR (TRIM(COALESCE(phone, '')) = '' AND LOWER(TRIM(username)) = ?))"
}

func prepareUserRow(row *models.User) {
	now := time.Now().UTC()
	if row.CreatedAt.IsZero() {
		row.CreatedAt = now
	}
	row.UpdatedAt = now
	if strings.TrimSpace(row.Status) == "" {
		row.Status = models.UserStatusActive
	}
	if strings.TrimSpace(row.Provider) == "" {
		row.Provider = models.UserProviderLocal
	}
}

func (d *Driver) CreateUser(ctx context.Context, row *models.User) (*models.User, error) {
	if d == nil || d.ORM == nil || row == nil {
		return nil, errors.New("sql: CreateUser invalid input")
	}
	prepareUserRow(row)
	_, err := d.ORM.NewInsert().Model(row).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return row, nil
}

func (d *Driver) GetUser(ctx context.Context, projectID, userID string) (*models.User, error) {
	if d == nil || d.ORM == nil {
		return nil, errors.New("sql: nil driver")
	}
	row := new(models.User)
	err := d.ORM.NewSelect().Model(row).
		Where("id = ?", userID).
		Where("project_id = ?", projectID).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return row, nil
}

func (d *Driver) GetUserByUsername(ctx context.Context, projectID, username string) (*models.User, error) {
	if d == nil || d.ORM == nil {
		return nil, errors.New("sql: nil driver")
	}
	u := strings.TrimSpace(username)
	if u == "" {
		return nil, nil
	}
	row := new(models.User)
	err := d.ORM.NewSelect().Model(row).
		Where("project_id = ?", projectID).
		Where("username = ?", u).
		Limit(1).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return row, nil
}

func (d *Driver) ListUsersByEmail(ctx context.Context, projectID, email string) ([]*models.User, error) {
	if d == nil || d.ORM == nil {
		return nil, errors.New("sql: nil driver")
	}
	e := strings.TrimSpace(strings.ToLower(email))
	if e == "" {
		return nil, nil
	}
	var rows []*models.User
	err := d.ORM.NewSelect().Model(&rows).
		Where("project_id = ?", projectID).
		Where("LOWER(TRIM(email)) = ?", e).
		Order("created_at ASC").
		Scan(ctx)
	return rows, err
}

func (d *Driver) ListUsersByPhone(ctx context.Context, projectID, phone string) ([]*models.User, error) {
	if d == nil || d.ORM == nil {
		return nil, errors.New("sql: nil driver")
	}
	norm := models.NormalizeUserPhoneKey(phone)
	if norm == "" {
		return nil, nil
	}
	var rows []*models.User
	err := d.ORM.NewSelect().Model(&rows).
		Where("project_id = ?", projectID).
		Where(phoneOrLegacyUsernameMatch(), norm, norm).
		Order("created_at ASC").
		Scan(ctx)
	return rows, err
}

func (d *Driver) ListUsersByGoogleSub(ctx context.Context, projectID, googleSub string) ([]*models.User, error) {
	if d == nil || d.ORM == nil {
		return nil, errors.New("sql: nil driver")
	}
	g := strings.TrimSpace(googleSub)
	if g == "" {
		return nil, nil
	}
	var rows []*models.User
	err := d.ORM.NewSelect().Model(&rows).
		Where("project_id = ?", projectID).
		Where("google_sub = ?", g).
		Order("created_at ASC").
		Scan(ctx)
	return rows, err
}

func (d *Driver) SearchProjectUsers(ctx context.Context, projectID string, limit, offset int) ([]*models.User, int, error) {
	if d == nil || d.ORM == nil {
		return nil, 0, errors.New("sql: nil driver")
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}
	count, err := d.ORM.NewSelect().Model((*models.User)(nil)).
		Where("project_id = ?", projectID).
		Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	var rows []*models.User
	err = d.ORM.NewSelect().Model(&rows).
		Where("project_id = ?", projectID).
		Order("created_at ASC").
		Limit(limit).
		Offset(offset).
		Scan(ctx)
	if err != nil {
		return nil, 0, err
	}
	return rows, count, nil
}

func (d *Driver) CountProjectUsersByRole(ctx context.Context, projectID string) (map[string]int, error) {
	if d == nil || d.ORM == nil {
		return nil, errors.New("sql: nil driver")
	}
	var rows []roleCountRow
	err := d.ORM.NewSelect().
		Model((*models.User)(nil)).
		Column("role").
		ColumnExpr("COUNT(*) AS count").
		Where("project_id = ?", projectID).
		Group("role").
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int, len(rows))
	for _, row := range rows {
		out[row.Role] = row.Count
	}
	return out, nil
}

type roleCountRow struct {
	Role  string `bun:"role"`
	Count int    `bun:"count"`
}

func (d *Driver) UpdateUser(ctx context.Context, row *models.User) error {
	if d == nil || d.ORM == nil || row == nil || row.ID == "" {
		return errors.New("sql: UpdateUser invalid input")
	}
	row.UpdatedAt = time.Now().UTC()
	_, err := d.ORM.NewUpdate().Model(row).
		Column("username", "email", "phone", "secret", "role", "provider", "google_sub", "status", "updated_at").
		Where("id = ? AND project_id = ?", row.ID, row.ProjectID).
		Exec(ctx)
	return err
}

func (d *Driver) DeleteUser(ctx context.Context, projectID, userID string) error {
	if d == nil || d.ORM == nil {
		return errors.New("sql: nil driver")
	}
	_, err := d.ORM.NewDelete().Model((*models.User)(nil)).
		Where("id = ?", userID).
		Where("project_id = ?", projectID).
		Exec(ctx)
	return err
}

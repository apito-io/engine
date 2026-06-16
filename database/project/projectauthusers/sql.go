package projectauthusers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect"
)

const usersTable = models.ProjectAuthUsersTableName

// SQLStore implements project DB app-user persistence for SQL engines.
type SQLStore struct {
	DB bun.IDB
}

func phoneOrLegacyUsernameMatch() string {
	return "(LOWER(TRIM(COALESCE(u.phone, ''))) = ? OR (TRIM(COALESCE(u.phone, '')) = '' AND LOWER(TRIM(u.username)) = ?))"
}

func prepareAuthUserRow(row *models.ProjectAuthUser) {
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
	if e := strings.TrimSpace(row.Email); e != "" {
		row.Email = strings.ToLower(e)
	}
	if p := strings.TrimSpace(row.Phone); p != "" {
		row.Phone = models.NormalizeUserPhoneKey(p)
	}
	row.GoogleSub = strings.TrimSpace(row.GoogleSub)
}

func tenantFilter(q *bun.SelectQuery, tenantID string) *bun.SelectQuery {
	if tid := strings.TrimSpace(tenantID); tid != "" {
		return q.Where("u.tenant_id = ?", tid)
	}
	return q
}

func (s *SQLStore) EnsureUsersTable(ctx context.Context) error {
	if s == nil || s.DB == nil {
		return errors.New("projectauthusers: nil store")
	}
	dialectName := ""
	if db, ok := s.DB.(*bun.DB); ok && db != nil {
		dialectName = db.Dialect().Name().String()
	}
	if err := execUsersDDL(ctx, s.DB, dialectName); err != nil {
		return err
	}
	return migrateLegacyProjectUsers(ctx, s.DB, dialectName)
}

func execUsersDDL(ctx context.Context, db bun.IDB, dialectName string) error {
	table := usersTable
	switch dialectName {
	case dialect.PG.String():
		_, err := db.ExecContext(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	id VARCHAR(36) NOT NULL PRIMARY KEY,
	tenant_id VARCHAR(36),
	username VARCHAR(255) NOT NULL,
	email VARCHAR(255),
	phone VARCHAR(64),
	secret TEXT,
	role VARCHAR(64) NOT NULL,
	provider VARCHAR(32) NOT NULL,
	google_sub VARCHAR(255),
	status VARCHAR(32) NOT NULL,
	created_at TIMESTAMPTZ,
	updated_at TIMESTAMPTZ
);`, table))
		if err != nil {
			return err
		}
		indexes := []string{
			fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_tenant_username ON %s (tenant_id, username)`, table),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_users_tenant ON %s (tenant_id)`, table),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_users_email ON %s (LOWER(TRIM(email)))`, table),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_users_google_sub ON %s (google_sub)`, table),
			fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique ON %s (LOWER(TRIM(email))) WHERE TRIM(COALESCE(email, '')) != ''`, table),
			fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_unique ON %s (phone) WHERE TRIM(COALESCE(phone, '')) != ''`, table),
			fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_google_sub_unique ON %s (google_sub) WHERE TRIM(COALESCE(google_sub, '')) != ''`, table),
		}
		for _, q := range indexes {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	case dialect.MySQL.String():
		_, err := db.ExecContext(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	id VARCHAR(36) NOT NULL PRIMARY KEY,
	tenant_id VARCHAR(36),
	username VARCHAR(255) NOT NULL,
	email VARCHAR(255),
	phone VARCHAR(64),
	secret TEXT,
	role VARCHAR(64) NOT NULL,
	provider VARCHAR(32) NOT NULL,
	google_sub VARCHAR(255),
	status VARCHAR(32) NOT NULL,
	created_at DATETIME,
	updated_at DATETIME,
	UNIQUE KEY idx_users_tenant_username (tenant_id, username),
	UNIQUE KEY idx_users_email_unique (email),
	UNIQUE KEY idx_users_phone_unique (phone),
	UNIQUE KEY idx_users_google_sub_unique (google_sub),
	KEY idx_users_tenant (tenant_id),
	KEY idx_users_email (email),
	KEY idx_users_google_sub (google_sub)
);`, table))
		return err
	default:
		// SQLite and libsql-compatible engines
		_, err := db.ExecContext(ctx, fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
	id VARCHAR(36) NOT NULL PRIMARY KEY,
	tenant_id VARCHAR(36),
	username VARCHAR(255) NOT NULL,
	email VARCHAR(255),
	phone VARCHAR(64),
	secret TEXT,
	role VARCHAR(64) NOT NULL,
	provider VARCHAR(32) NOT NULL,
	google_sub VARCHAR(255),
	status VARCHAR(32) NOT NULL,
	created_at TEXT,
	updated_at TEXT
);`, table))
		if err != nil {
			return err
		}
		indexes := []string{
			fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_tenant_username ON %s (tenant_id, username)`, table),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_users_tenant ON %s (tenant_id)`, table),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_users_email ON %s (email)`, table),
			fmt.Sprintf(`CREATE INDEX IF NOT EXISTS idx_users_google_sub ON %s (google_sub)`, table),
			fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email_unique ON %s (email) WHERE TRIM(COALESCE(email, '')) != ''`, table),
			fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone_unique ON %s (phone) WHERE TRIM(COALESCE(phone, '')) != ''`, table),
			fmt.Sprintf(`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_google_sub_unique ON %s (google_sub) WHERE TRIM(COALESCE(google_sub, '')) != ''`, table),
		}
		for _, q := range indexes {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	}
}

func migrateLegacyProjectUsers(ctx context.Context, db bun.IDB, dialectName string) error {
	if !legacyProjectUsersTableExists(ctx, db, dialectName) {
		return nil
	}
	switch dialectName {
	case dialect.PG.String():
		_, err := db.ExecContext(ctx, `
INSERT INTO users (id, tenant_id, username, email, phone, secret, role, provider, google_sub, status, created_at, updated_at)
SELECT pu.id,
  NULLIF(TRIM(pu.tenant_id), ''),
  pu.username,
  pu.email,
  pu.phone,
  pu.secret,
  pu.role,
  pu.provider,
  pu.google_sub,
  pu.status,
  pu.created_at,
  pu.updated_at
FROM project_users pu
WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = pu.id);`)
		return err
	case dialect.MySQL.String():
		_, err := db.ExecContext(ctx, `
INSERT INTO users (id, tenant_id, username, email, phone, secret, role, provider, google_sub, status, created_at, updated_at)
SELECT pu.id,
  NULLIF(TRIM(pu.tenant_id), ''),
  pu.username,
  pu.email,
  pu.phone,
  pu.secret,
  pu.role,
  pu.provider,
  pu.google_sub,
  pu.status,
  pu.created_at,
  pu.updated_at
FROM project_users pu
WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = pu.id);`)
		return err
	default:
		_, err := db.ExecContext(ctx, `
INSERT INTO users (id, tenant_id, username, email, phone, secret, role, provider, google_sub, status, created_at, updated_at)
SELECT pu.id,
  NULLIF(TRIM(pu.tenant_id), ''),
  pu.username,
  pu.email,
  pu.phone,
  pu.secret,
  pu.role,
  pu.provider,
  pu.google_sub,
  pu.status,
  pu.created_at,
  pu.updated_at
FROM project_users pu
WHERE NOT EXISTS (SELECT 1 FROM users u WHERE u.id = pu.id);`)
		return err
	}
}

func legacyProjectUsersTableExists(ctx context.Context, db bun.IDB, dialectName string) bool {
	var n int
	var err error
	switch dialectName {
	case dialect.PG.String():
		err = db.QueryRowContext(ctx, `SELECT COUNT(*)::int FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'project_users'`).Scan(&n)
	case dialect.MySQL.String():
		err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'project_users'`).Scan(&n)
	default:
		err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='project_users'`).Scan(&n)
	}
	return err == nil && n > 0
}

func (s *SQLStore) CreateProjectAuthUser(ctx context.Context, row *models.ProjectAuthUser) (*models.ProjectAuthUser, error) {
	if s == nil || s.DB == nil || row == nil {
		return nil, errors.New("projectauthusers: CreateProjectAuthUser invalid input")
	}
	if err := s.EnsureUsersTable(ctx); err != nil {
		return nil, err
	}
	prepareAuthUserRow(row)
	_, err := s.DB.NewInsert().Model(row).Exec(ctx)
	if err != nil {
		return nil, mapAuthUserUniqueViolation(err)
	}
	return row, nil
}

func (s *SQLStore) GetProjectAuthUser(ctx context.Context, userID string) (*models.ProjectAuthUser, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("projectauthusers: nil store")
	}
	if err := s.EnsureUsersTable(ctx); err != nil {
		return nil, err
	}
	row := new(models.ProjectAuthUser)
	err := s.DB.NewSelect().Model(row).Where("id = ?", userID).Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return row, nil
}

func (s *SQLStore) GetProjectAuthUserByUsername(ctx context.Context, username string) (*models.ProjectAuthUser, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("projectauthusers: nil store")
	}
	u := strings.TrimSpace(username)
	if u == "" {
		return nil, nil
	}
	if err := s.EnsureUsersTable(ctx); err != nil {
		return nil, err
	}
	row := new(models.ProjectAuthUser)
	err := s.DB.NewSelect().Model(row).Where("username = ?", u).Limit(1).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return row, nil
}

func (s *SQLStore) ListProjectAuthUsersByEmail(ctx context.Context, tenantID, email string) ([]*models.ProjectAuthUser, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("projectauthusers: nil store")
	}
	e := strings.TrimSpace(strings.ToLower(email))
	if e == "" {
		return nil, nil
	}
	if err := s.EnsureUsersTable(ctx); err != nil {
		return nil, err
	}
	var rows []*models.ProjectAuthUser
	q := s.DB.NewSelect().Model(&rows).Where("LOWER(TRIM(u.email)) = ?", e).Order("created_at ASC")
	q = tenantFilter(q, tenantID)
	err := q.Scan(ctx)
	return rows, err
}

func (s *SQLStore) ListProjectAuthUsersByPhone(ctx context.Context, tenantID, phone string) ([]*models.ProjectAuthUser, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("projectauthusers: nil store")
	}
	norm := models.NormalizeUserPhoneKey(phone)
	if norm == "" {
		return nil, nil
	}
	if err := s.EnsureUsersTable(ctx); err != nil {
		return nil, err
	}
	var rows []*models.ProjectAuthUser
	q := s.DB.NewSelect().Model(&rows).Where(phoneOrLegacyUsernameMatch(), norm, norm).Order("created_at ASC")
	q = tenantFilter(q, tenantID)
	err := q.Scan(ctx)
	return rows, err
}

func (s *SQLStore) ListProjectAuthUsersByGoogleSub(ctx context.Context, tenantID, googleSub string) ([]*models.ProjectAuthUser, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("projectauthusers: nil store")
	}
	g := strings.TrimSpace(googleSub)
	if g == "" {
		return nil, nil
	}
	if err := s.EnsureUsersTable(ctx); err != nil {
		return nil, err
	}
	var rows []*models.ProjectAuthUser
	q := s.DB.NewSelect().Model(&rows).Where("google_sub = ?", g).Order("created_at ASC")
	q = tenantFilter(q, tenantID)
	err := q.Scan(ctx)
	return rows, err
}

type roleCountRow struct {
	Role  string `bun:"role"`
	Count int    `bun:"count"`
}

func (s *SQLStore) CountProjectAuthUsersByRole(ctx context.Context, tenantID string) (map[string]int, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("projectauthusers: nil store")
	}
	if err := s.EnsureUsersTable(ctx); err != nil {
		return nil, err
	}
	var rows []roleCountRow
	q := s.DB.NewSelect().
		Model((*models.ProjectAuthUser)(nil)).
		Column("role").
		ColumnExpr("COUNT(*) AS count").
		Group("role")
	q = tenantFilter(q, tenantID)
	if err := q.Scan(ctx, &rows); err != nil {
		return nil, err
	}
	out := make(map[string]int, len(rows))
	for _, row := range rows {
		out[row.Role] = row.Count
	}
	return out, nil
}

func (s *SQLStore) SearchProjectAuthUsers(ctx context.Context, tenantID string, limit, offset int) ([]*models.ProjectAuthUser, int, error) {
	if s == nil || s.DB == nil {
		return nil, 0, errors.New("projectauthusers: nil store")
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
	if err := s.EnsureUsersTable(ctx); err != nil {
		return nil, 0, err
	}
	countQ := s.DB.NewSelect().Model((*models.ProjectAuthUser)(nil))
	countQ = tenantFilter(countQ, tenantID)
	count, err := countQ.Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	var rows []*models.ProjectAuthUser
	listQ := s.DB.NewSelect().Model(&rows).Order("created_at ASC").Limit(limit).Offset(offset)
	listQ = tenantFilter(listQ, tenantID)
	if err := listQ.Scan(ctx); err != nil {
		return nil, 0, err
	}
	return rows, count, nil
}

func (s *SQLStore) UpdateProjectAuthUser(ctx context.Context, row *models.ProjectAuthUser) error {
	if s == nil || s.DB == nil || row == nil || row.ID == "" {
		return errors.New("projectauthusers: UpdateProjectAuthUser invalid input")
	}
	if err := s.EnsureUsersTable(ctx); err != nil {
		return err
	}
	row.UpdatedAt = time.Now().UTC()
	cols := []string{"username", "email", "phone", "secret", "role", "provider", "google_sub", "status", "updated_at"}
	if tid := strings.TrimSpace(row.TenantID); tid != "" {
		cols = append(cols, "tenant_id")
	}
	_, err := s.DB.NewUpdate().Model(row).Column(cols...).Where("id = ?", row.ID).Exec(ctx)
	return mapAuthUserUniqueViolation(err)
}

func (s *SQLStore) DeleteProjectAuthUser(ctx context.Context, userID string) error {
	if s == nil || s.DB == nil {
		return errors.New("projectauthusers: nil store")
	}
	if err := s.EnsureUsersTable(ctx); err != nil {
		return err
	}
	_, err := s.DB.NewDelete().Model((*models.ProjectAuthUser)(nil)).Where("id = ?", userID).Exec(ctx)
	return err
}

package models

import (
	"strings"
	"time"
)

const (
	// ProjectAuthUsersTableName is the reserved project DB table for application end-users.
	ProjectAuthUsersTableName = "users"
)

// ProjectAuthUser is an app end-user row stored in the project database (table: users).
// Project scope is implied by the database connection; tenant_id is used for SaaS shared-DB isolation.
type ProjectAuthUser struct {
	ORMBase `bun:"table:users,alias:u"`

	ID        string `bun:"id,pk" json:"id"`
	TenantID  string `bun:"tenant_id,nullzero" json:"tenant_id,omitempty"`
	Username  string `bun:"username,notnull" json:"-"`
	Email     string `bun:"email" json:"email,omitempty"`
	Phone     string `bun:"phone,nullzero" json:"phone,omitempty"`
	Secret    string `bun:"secret" json:"-"`
	Role      string `bun:"role,notnull" json:"role"`
	Provider  string `bun:"provider,notnull" json:"provider"`
	GoogleSub string `bun:"google_sub" json:"google_sub,omitempty"`
	Status    string `bun:"status,notnull" json:"status"`
	CreatedAt time.Time `bun:"created_at,nullzero" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero" json:"updated_at"`
}

// UserFromProjectAuthUser maps a project DB row to the legacy User shape (ProjectID supplied by caller).
func UserFromProjectAuthUser(projectID string, row *ProjectAuthUser) *User {
	if row == nil {
		return nil
	}
	return &User{
		ID:        row.ID,
		ProjectID: projectID,
		Username:  row.Username,
		Email:     row.Email,
		Phone:     row.Phone,
		Secret:    row.Secret,
		Role:      row.Role,
		Provider:  row.Provider,
		GoogleSub: row.GoogleSub,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

// ProjectAuthUserFromUser converts a legacy User into a project DB row.
// tenantID is stored only for SaaS shared-DB projects; leave empty for general or per-tenant DB routing.
func ProjectAuthUserFromUser(u *User, tenantID string) *ProjectAuthUser {
	if u == nil {
		return nil
	}
	row := &ProjectAuthUser{
		ID:        u.ID,
		Username:  u.Username,
		Email:     u.Email,
		Phone:     u.Phone,
		Secret:    u.Secret,
		Role:      u.Role,
		Provider:  u.Provider,
		GoogleSub: u.GoogleSub,
		Status:    u.Status,
		CreatedAt: u.CreatedAt,
		UpdatedAt: u.UpdatedAt,
	}
	if tid := strings.TrimSpace(tenantID); tid != "" {
		row.TenantID = tid
	}
	return row
}

// ProjectAuthUserToPublicMap returns a GraphQL-safe map (no secret).
func ProjectAuthUserToPublicMap(u *ProjectAuthUser) map[string]interface{} {
	if u == nil {
		return nil
	}
	m := map[string]interface{}{
		"id":         u.ID,
		"email":      u.Email,
		"phone":      u.Phone,
		"role":       u.Role,
		"provider":   u.Provider,
		"status":     u.Status,
		"created_at": u.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": u.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if tid := strings.TrimSpace(u.TenantID); tid != "" {
		m["tenant_id"] = tid
	}
	return m
}

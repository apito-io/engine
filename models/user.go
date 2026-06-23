package models

import (
	"strings"
	"time"
)

// Project end-user accounts (application auth). Not SystemUser (console operators).
const (
	// ProjectUsersTableName is the legacy system DB table / Mongo collection for app end-users (migration source).
	ProjectUsersTableName = "project_users"

	UserStatusActive     = "active"
	UserStatusSuspended  = "suspended"
	UserProviderLocal    = "local"
	UserProviderGoogle   = "google"
	UserDefaultRoleAdmin = "admin"
)

// NormalizeUserPhoneKey lowercases and trims phone for lookups (case-insensitive sign-in).
func NormalizeUserPhoneKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// User is a project-scoped application end-user (legacy system DB: project_users; target store: project DB users).
type User struct {
	ORMBase `bun:"table:project_users,alias:pu"`

	ID        string `bun:"id,pk" json:"id" bson:"_id,omitempty"`
	ProjectID string `bun:"project_id,notnull" json:"project_id" bson:"project_id,omitempty"`

	// Username is an internal stable key for SQL uniqueness (project_id, username).
	Username string `bun:"username,notnull" json:"-" bson:"username,omitempty"`
	Email    string `bun:"email" json:"email,omitempty" bson:"email,omitempty"`
	Phone    string `bun:"phone,nullzero" json:"phone,omitempty" bson:"phone,omitempty"`
	Secret   string `bun:"secret" json:"-" bson:"secret,omitempty"`

	Role      string `bun:"role,notnull" json:"role" bson:"role,omitempty"`
	Provider  string `bun:"provider,notnull" json:"provider" bson:"provider,omitempty"`
	GoogleSub string `bun:"google_sub" json:"google_sub,omitempty" bson:"google_sub,omitempty"`

	Status string `bun:"status,notnull" json:"status" bson:"status,omitempty"`

	CreatedAt time.Time `bun:"created_at,nullzero" json:"created_at" bson:"created_at,omitempty"`
	UpdatedAt time.Time `bun:"updated_at,nullzero" json:"updated_at" bson:"updated_at,omitempty"`
}

// UserToPublicMap returns a GraphQL-safe map (no secret).
func UserToPublicMap(u *User) map[string]interface{} {
	if u == nil {
		return nil
	}
	return map[string]interface{}{
		"id":         u.ID,
		"email":      u.Email,
		"phone":      u.Phone,
		"role":       u.Role,
		"provider":   u.Provider,
		"status":     u.Status,
		"created_at": u.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": u.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

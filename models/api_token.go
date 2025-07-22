package models

import (
	"time"
)

type JWTTokens struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`

	TenantID string `json:"tenant_id"`
}

type TokenClaims struct {
	ProjectID     string `json:"project_id"`
	TokenUniqueID string `json:"token_unique_id"`

	Role   string `json:"role"`
	UserID string `json:"user_id"`
	Email  string `json:"email"`

	IsSuperAdmin  bool `json:"is_super_admin"`
	IsProjectUser bool `json:"is_project_user"`
	IsReadOnly    bool `json:"is_read_only"`

	AccessPermissions []string `json:"access_permissions"`

	TenantID       string `json:"tenant_id"`        // used for SaaS app only
	TokenType      string `json:"token_type"`       // access_token or id_token
	PaymentDueDate string `json:"payment_due_date"` // used if the user dosnt pay

	ExpireAt time.Time `json:"expire_at"` // unix timestamp

	Scopes []string `json:"scopes"` // ['read', 'write']
}

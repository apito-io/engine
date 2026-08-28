package models

type JWTTokens struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	Ext          map[string]interface{} `json:"ext,omitempty"`
}

type TokenClaims struct {
	ProjectID     string `json:"project_id"`
	TokenUniqueID string `json:"token_unique_id"`

	Role   string `json:"role"`
	UserID string `json:"user_id"`
	Email  string `json:"email"`

	IsProjectUser bool `json:"is_project_user"`
	IsReadOnly    bool `json:"is_read_only"`

	// IsSuperAdmin is the platform-operator claim. Set on console JWTs and apt_
	// tokens from the issuer row; never set on project ak_ tokens.
	IsSuperAdmin bool `json:"is_super_admin"`

	AccessPermissions []string `json:"access_permissions"`

	TokenType      string `json:"token_type"`       // access_token or id_token
	PaymentDueDate string `json:"payment_due_date"` // used if the user dosnt pay

	ExpireAt int64 `json:"expire_at"` // unix timestamp

	ProjectIDs []string `json:"project_ids"` // ['project_id_1', 'project_id_2']
	Scopes     []string `json:"scopes"`      // ['read', 'write']
	Ext        map[string]interface{} `json:"ext,omitempty"`
}

package models

// Access token grant / status constants.
const (
	AccessTokenStatusActive  = "active"
	AccessTokenStatusRevoked = "revoked"
	AccessTokenStatusExpired = "expired"

	AccessTokenProjectGrantSelected = "selected"
	AccessTokenProjectGrantAllAdmin = "all_admin_projects"

	AccessTokenTenantGrantAll      = "all"
	AccessTokenTenantGrantSelected = "selected"
	AccessTokenTenantGrantNone     = "none"

	AccessTokenPrefix    = "apt_"
	ApitoProjectIDHeader = "X-Apito-Project-Id"
	ApitoTenantIDHeader  = "X-Apito-Tenant-ID"
)

// AccessTokenRecord is a persisted, revocable system-user automation token.
// The raw secret is never stored — only SecretHash. Raw value is returned once at mint.
type AccessTokenRecord struct {
	ID           string `json:"id,omitempty"`
	SecretHash   string `json:"secret_hash,omitempty"`
	SecretPrefix string `json:"secret_prefix,omitempty"`
	Name         string `json:"name,omitempty"`
	Description  string `json:"description,omitempty"`
	IssuerUserID string `json:"issuer_user_id,omitempty"`
	Status       string `json:"status,omitempty"`
	Preset       string `json:"preset,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
	LastUsedAt   string `json:"last_used_at,omitempty"`
	LastUsedIP   string `json:"last_used_ip,omitempty"`
	LastUsedUA   string `json:"last_used_user_agent,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	RevokedAt    string `json:"revoked_at,omitempty"`
	RevokedBy    string `json:"revoked_by,omitempty"`

	ProjectGrantMode string   `json:"project_grant_mode,omitempty"`
	ProjectIDs       []string `json:"project_ids,omitempty"`

	TenantGrantMode string              `json:"tenant_grant_mode,omitempty"`
	TenantIDs       map[string][]string `json:"tenant_ids,omitempty"` // projectID -> tenant IDs

	Capabilities []string `json:"capabilities,omitempty"`
	AllowedCIDRs []string `json:"allowed_cidrs,omitempty"`
}

// AccessTokenPublic is the safe list/detail projection (never includes SecretHash).
type AccessTokenPublic struct {
	ID               string              `json:"id"`
	SecretPrefix     string              `json:"secret_prefix,omitempty"`
	Name             string              `json:"name,omitempty"`
	Description      string              `json:"description,omitempty"`
	IssuerUserID     string              `json:"issuer_user_id,omitempty"`
	Status           string              `json:"status,omitempty"`
	Preset           string              `json:"preset,omitempty"`
	ExpiresAt        string              `json:"expires_at,omitempty"`
	LastUsedAt       string              `json:"last_used_at,omitempty"`
	LastUsedIP       string              `json:"last_used_ip,omitempty"`
	LastUsedUA       string              `json:"last_used_user_agent,omitempty"`
	CreatedAt        string              `json:"created_at,omitempty"`
	RevokedAt        string              `json:"revoked_at,omitempty"`
	ProjectGrantMode string              `json:"project_grant_mode,omitempty"`
	ProjectIDs       []string            `json:"project_ids,omitempty"`
	TenantGrantMode  string              `json:"tenant_grant_mode,omitempty"`
	TenantIDs        map[string][]string `json:"tenant_ids,omitempty"`
	Capabilities     []string            `json:"capabilities,omitempty"`
	AllowedCIDRs     []string            `json:"allowed_cidrs,omitempty"`
	CapabilityCount  int                 `json:"capability_count,omitempty"`
}

// ToPublic strips secrets from an AccessTokenRecord.
func (r *AccessTokenRecord) ToPublic() *AccessTokenPublic {
	if r == nil {
		return nil
	}
	return &AccessTokenPublic{
		ID:               r.ID,
		SecretPrefix:     r.SecretPrefix,
		Name:             r.Name,
		Description:      r.Description,
		IssuerUserID:     r.IssuerUserID,
		Status:           r.Status,
		Preset:           r.Preset,
		ExpiresAt:        r.ExpiresAt,
		LastUsedAt:       r.LastUsedAt,
		LastUsedIP:       r.LastUsedIP,
		LastUsedUA:       r.LastUsedUA,
		CreatedAt:        r.CreatedAt,
		RevokedAt:        r.RevokedAt,
		ProjectGrantMode: r.ProjectGrantMode,
		ProjectIDs:       append([]string(nil), r.ProjectIDs...),
		TenantGrantMode:  r.TenantGrantMode,
		TenantIDs:        cloneTenantMap(r.TenantIDs),
		Capabilities:     append([]string(nil), r.Capabilities...),
		AllowedCIDRs:     append([]string(nil), r.AllowedCIDRs...),
		CapabilityCount:  len(r.Capabilities),
	}
}

func cloneTenantMap(in map[string][]string) map[string][]string {
	if in == nil {
		return nil
	}
	out := make(map[string][]string, len(in))
	for k, v := range in {
		out[k] = append([]string(nil), v...)
	}
	return out
}

// AccessPrincipal is request-scoped identity derived from an apt_ token.
type AccessPrincipal struct {
	TokenID          string
	IssuerUserID     string
	ProjectGrantMode string
	ProjectIDs       []string
	TenantGrantMode  string
	TenantIDs        map[string][]string
	Capabilities     []string
	AllowedCIDRs     []string
	TokenType        string // always "access_token"
}

// CreateAccessTokenRequest is the REST mint payload.
type CreateAccessTokenRequest struct {
	Name              string              `json:"name"`
	Description       string              `json:"description"`
	Duration          string              `json:"duration"` // YYYY-MM-DD or empty for default/never
	Preset            string              `json:"preset"`
	ProjectGrantMode  string              `json:"project_grant_mode"`
	ProjectIDs        []string            `json:"project_ids"`
	TenantGrantMode   string              `json:"tenant_grant_mode"`
	TenantIDs         map[string][]string `json:"tenant_ids"`
	Capabilities      []string            `json:"capabilities"`
	AllowedCIDRs      []string            `json:"allowed_cidrs"`
	AcknowledgeDanger bool                `json:"acknowledge_danger"`
}

// RotateAccessTokenRequest rotates an existing token id (same grants, new secret).
type RotateAccessTokenRequest struct {
	ID string `json:"id"`
}

// RevokeAccessTokenRequest revokes by token id (preferred) or raw token (one-shot).
type RevokeAccessTokenRequest struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

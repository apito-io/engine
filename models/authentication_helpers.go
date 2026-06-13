package models

import (
	"errors"
	"strings"
)

// BoolPtr returns a pointer to b.
func BoolPtr(b bool) *bool { return &b }

// GeneralAuthEffective is true when general (password) login should be allowed.
func GeneralAuthEffective(p *Project) bool {
	if p == nil {
		return false
	}
	if p.AuthenticationSettings == nil {
		return true
	}
	if p.AuthenticationSettings.EnableGeneralAuth == nil {
		return true
	}
	return *p.AuthenticationSettings.EnableGeneralAuth
}

// GoogleAuthEffective is true when Google login should be allowed.
func GoogleAuthEffective(p *Project) bool {
	if p == nil {
		return false
	}
	cid := strings.TrimSpace(GoogleOAuthClientID(p))
	if p.AuthenticationSettings == nil {
		return cid != ""
	}
	if p.AuthenticationSettings.EnableGoogleAuth != nil {
		return *p.AuthenticationSettings.EnableGoogleAuth && cid != ""
	}
	return cid != ""
}

// GoogleOAuthClientID returns the OAuth client ID from AuthenticationSettings (trimmed).
func GoogleOAuthClientID(p *Project) string {
	if p == nil || p.AuthenticationSettings == nil {
		return ""
	}
	return strings.TrimSpace(p.AuthenticationSettings.GoogleClientID)
}

// GoogleOAuthClientSecret returns the OAuth client secret from AuthenticationSettings (trimmed).
func GoogleOAuthClientSecret(p *Project) string {
	if p == nil || p.AuthenticationSettings == nil {
		return ""
	}
	return strings.TrimSpace(p.AuthenticationSettings.GoogleClientSecret)
}

// GoogleOAuthRedirectURI returns the configured authorized redirect URI for Google OAuth (code flow).
func GoogleOAuthRedirectURI(p *Project) string {
	if p == nil || p.AuthenticationSettings == nil {
		return ""
	}
	return strings.TrimSpace(p.AuthenticationSettings.GoogleOAuthRedirectURI)
}

// GeneralIdentifierMethod returns "email", "phone", or "email" as default when unset/invalid.
func GeneralIdentifierMethod(p *Project) string {
	if p == nil || p.AuthenticationSettings == nil {
		return "email"
	}
	switch strings.ToLower(strings.TrimSpace(p.AuthenticationSettings.GeneralAuthenticationMethod)) {
	case "phone":
		return "phone"
	default:
		return "email"
	}
}

// HasGoogleClientSecretConfigured reports whether a non-empty secret is stored.
func HasGoogleClientSecretConfigured(p *Project) bool {
	return GoogleOAuthClientSecret(p) != ""
}

// GoogleOAuthCodeExchangeReady reports whether server-side OAuth code exchange can run.
func GoogleOAuthCodeExchangeReady(p *Project) bool {
	if !GoogleAuthEffective(p) {
		return false
	}
	cid := GoogleOAuthClientID(p)
	sec := GoogleOAuthClientSecret(p)
	rd := GoogleOAuthRedirectURI(p)
	return cid != "" && sec != "" && strings.TrimSpace(rd) != ""
}

// Deprecated: use GoogleOAuthCodeExchangeReady.
func TenantGoogleOAuthCodeExchangeReady(p *Project) bool {
	return GoogleOAuthCodeExchangeReady(p)
}

// DefaultRegistrationRoleConfigured returns the stored default registration role (trimmed, may be empty).
func DefaultRegistrationRoleConfigured(p *Project) string {
	if p == nil || p.AuthenticationSettings == nil {
		return ""
	}
	return strings.TrimSpace(p.AuthenticationSettings.DefaultRegistrationRole)
}

// RegistrationDefaultRole returns the role assigned to new users when none is specified.
// Empty configuration falls back to "none".
func RegistrationDefaultRole(p *Project) string {
	if role := DefaultRegistrationRoleConfigured(p); role != "" {
		return role
	}
	return "none"
}

// ValidateRegistrationDefaultRole ensures a non-empty role exists on the project and is not admin.
func ValidateRegistrationDefaultRole(project *Project, role string) error {
	role = strings.TrimSpace(role)
	if role == "" {
		return nil
	}
	if project == nil || project.Roles == nil {
		return errors.New("default_registration_role: project roles not loaded")
	}
	r, ok := project.Roles[role]
	if !ok {
		return errors.New("default_registration_role: role does not exist on this project")
	}
	if r != nil && (r.IsAdmin || strings.EqualFold(role, "admin")) {
		return errors.New("default_registration_role: admin role cannot be used as the default registration role")
	}
	return nil
}

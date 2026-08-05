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

// OAuthProviderCredentials holds trimmed client credentials for one provider.
type OAuthProviderCredentials struct {
	ClientID     string
	ClientSecret string
	RedirectURI  string
	Enable       *bool
}

// OAuthCredentials returns credentials for a provider from project settings.
func OAuthCredentials(p *Project, provider OAuthProviderID) OAuthProviderCredentials {
	var out OAuthProviderCredentials
	if p == nil || p.AuthenticationSettings == nil {
		return out
	}
	a := p.AuthenticationSettings
	switch provider {
	case OAuthProviderGoogle:
		out = OAuthProviderCredentials{
			ClientID: a.GoogleClientID, ClientSecret: a.GoogleClientSecret,
			RedirectURI: a.GoogleOAuthRedirectURI, Enable: a.EnableGoogleAuth,
		}
	case OAuthProviderFacebook:
		out = OAuthProviderCredentials{
			ClientID: a.FacebookClientID, ClientSecret: a.FacebookClientSecret,
			RedirectURI: a.FacebookOAuthRedirectURI, Enable: a.EnableFacebookAuth,
		}
	case OAuthProviderGithub:
		out = OAuthProviderCredentials{
			ClientID: a.GithubClientID, ClientSecret: a.GithubClientSecret,
			RedirectURI: a.GithubOAuthRedirectURI, Enable: a.EnableGithubAuth,
		}
	case OAuthProviderX:
		out = OAuthProviderCredentials{
			ClientID: a.XClientID, ClientSecret: a.XClientSecret,
			RedirectURI: a.XOAuthRedirectURI, Enable: a.EnableXAuth,
		}
	case OAuthProviderLinkedin:
		out = OAuthProviderCredentials{
			ClientID: a.LinkedinClientID, ClientSecret: a.LinkedinClientSecret,
			RedirectURI: a.LinkedinOAuthRedirectURI, Enable: a.EnableLinkedinAuth,
		}
	}
	out.ClientID = strings.TrimSpace(out.ClientID)
	out.ClientSecret = strings.TrimSpace(out.ClientSecret)
	out.RedirectURI = strings.TrimSpace(out.RedirectURI)
	return out
}

// OAuthAuthEffective is true when the provider login should be allowed.
func OAuthAuthEffective(p *Project, provider OAuthProviderID) bool {
	cred := OAuthCredentials(p, provider)
	if cred.ClientID == "" {
		return false
	}
	if cred.Enable != nil {
		return *cred.Enable
	}
	// Google legacy: enabled when client id present and flag unset.
	return provider == OAuthProviderGoogle
}

// OAuthCodeExchangeReady reports whether server-side OAuth code exchange can run.
func OAuthCodeExchangeReady(p *Project, provider OAuthProviderID) bool {
	if !OAuthAuthEffective(p, provider) {
		return false
	}
	cred := OAuthCredentials(p, provider)
	return cred.ClientID != "" && cred.ClientSecret != "" && cred.RedirectURI != ""
}

// HasOAuthClientSecretConfigured reports whether a non-empty secret is stored.
func HasOAuthClientSecretConfigured(p *Project, provider OAuthProviderID) bool {
	return OAuthCredentials(p, provider).ClientSecret != ""
}

// GoogleAuthEffective is true when Google login should be allowed.
func GoogleAuthEffective(p *Project) bool {
	return OAuthAuthEffective(p, OAuthProviderGoogle)
}

// GoogleOAuthClientID returns the OAuth client ID from AuthenticationSettings (trimmed).
func GoogleOAuthClientID(p *Project) string {
	return OAuthCredentials(p, OAuthProviderGoogle).ClientID
}

// GoogleOAuthClientSecret returns the OAuth client secret from AuthenticationSettings (trimmed).
func GoogleOAuthClientSecret(p *Project) string {
	return OAuthCredentials(p, OAuthProviderGoogle).ClientSecret
}

// GoogleOAuthRedirectURI returns the configured authorized redirect URI for Google OAuth (code flow).
func GoogleOAuthRedirectURI(p *Project) string {
	return OAuthCredentials(p, OAuthProviderGoogle).RedirectURI
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
	return HasOAuthClientSecretConfigured(p, OAuthProviderGoogle)
}

// GoogleOAuthCodeExchangeReady reports whether server-side OAuth code exchange can run.
func GoogleOAuthCodeExchangeReady(p *Project) bool {
	return OAuthCodeExchangeReady(p, OAuthProviderGoogle)
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

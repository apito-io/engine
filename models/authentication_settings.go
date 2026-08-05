package models

import "strings"

// AuthenticationSettings holds per-project sign-in configuration (all project types).
type AuthenticationSettings struct {
	EnableGeneralAuth           *bool  `json:"enable_general_auth,omitempty" firestore:"enable_general_auth,omitempty" bson:"enable_general_auth,omitempty"`
	EnableGoogleAuth            *bool  `json:"enable_google_auth,omitempty" firestore:"enable_google_auth,omitempty" bson:"enable_google_auth,omitempty"`
	EnableFacebookAuth          *bool  `json:"enable_facebook_auth,omitempty" firestore:"enable_facebook_auth,omitempty" bson:"enable_facebook_auth,omitempty"`
	EnableGithubAuth            *bool  `json:"enable_github_auth,omitempty" firestore:"enable_github_auth,omitempty" bson:"enable_github_auth,omitempty"`
	EnableXAuth                 *bool  `json:"enable_x_auth,omitempty" firestore:"enable_x_auth,omitempty" bson:"enable_x_auth,omitempty"`
	EnableLinkedinAuth          *bool  `json:"enable_linkedin_auth,omitempty" firestore:"enable_linkedin_auth,omitempty" bson:"enable_linkedin_auth,omitempty"`
	GeneralAuthenticationMethod string `json:"general_authentication_method,omitempty" firestore:"general_authentication_method,omitempty" bson:"general_authentication_method,omitempty"`
	GoogleClientID              string `json:"google_client_id,omitempty" firestore:"google_client_id,omitempty" bson:"google_client_id,omitempty"`
	GoogleClientSecret          string `json:"google_client_secret,omitempty" firestore:"google_client_secret,omitempty" bson:"google_client_secret,omitempty"`
	GoogleOAuthRedirectURI      string `json:"google_oauth_redirect_uri,omitempty" firestore:"google_oauth_redirect_uri,omitempty" bson:"google_oauth_redirect_uri,omitempty"`
	FacebookClientID            string `json:"facebook_client_id,omitempty" firestore:"facebook_client_id,omitempty" bson:"facebook_client_id,omitempty"`
	FacebookClientSecret        string `json:"facebook_client_secret,omitempty" firestore:"facebook_client_secret,omitempty" bson:"facebook_client_secret,omitempty"`
	FacebookOAuthRedirectURI    string `json:"facebook_oauth_redirect_uri,omitempty" firestore:"facebook_oauth_redirect_uri,omitempty" bson:"facebook_oauth_redirect_uri,omitempty"`
	GithubClientID              string `json:"github_client_id,omitempty" firestore:"github_client_id,omitempty" bson:"github_client_id,omitempty"`
	GithubClientSecret          string `json:"github_client_secret,omitempty" firestore:"github_client_secret,omitempty" bson:"github_client_secret,omitempty"`
	GithubOAuthRedirectURI      string `json:"github_oauth_redirect_uri,omitempty" firestore:"github_oauth_redirect_uri,omitempty" bson:"github_oauth_redirect_uri,omitempty"`
	XClientID                   string `json:"x_client_id,omitempty" firestore:"x_client_id,omitempty" bson:"x_client_id,omitempty"`
	XClientSecret               string `json:"x_client_secret,omitempty" firestore:"x_client_secret,omitempty" bson:"x_client_secret,omitempty"`
	XOAuthRedirectURI           string `json:"x_oauth_redirect_uri,omitempty" firestore:"x_oauth_redirect_uri,omitempty" bson:"x_oauth_redirect_uri,omitempty"`
	LinkedinClientID            string `json:"linkedin_client_id,omitempty" firestore:"linkedin_client_id,omitempty" bson:"linkedin_client_id,omitempty"`
	LinkedinClientSecret        string `json:"linkedin_client_secret,omitempty" firestore:"linkedin_client_secret,omitempty" bson:"linkedin_client_secret,omitempty"`
	LinkedinOAuthRedirectURI    string `json:"linkedin_oauth_redirect_uri,omitempty" firestore:"linkedin_oauth_redirect_uri,omitempty" bson:"linkedin_oauth_redirect_uri,omitempty"`
	DefaultRegistrationRole     string `json:"default_registration_role,omitempty" firestore:"default_registration_role,omitempty" bson:"default_registration_role,omitempty"`
}

// EnsureAuthenticationSettings returns a non-nil AuthenticationSettings pointer on Project.
func EnsureAuthenticationSettings(p *Project) *AuthenticationSettings {
	if p == nil {
		return nil
	}
	if p.AuthenticationSettings == nil {
		p.AuthenticationSettings = &AuthenticationSettings{}
	}
	return p.AuthenticationSettings
}

// OAuthProviderID is a project app-user OAuth provider key.
type OAuthProviderID string

const (
	OAuthProviderGoogle   OAuthProviderID = "google"
	OAuthProviderFacebook OAuthProviderID = "facebook"
	OAuthProviderGithub   OAuthProviderID = "github"
	OAuthProviderX        OAuthProviderID = "x"
	OAuthProviderLinkedin OAuthProviderID = "linkedin"
)

// ParseOAuthProviderID normalizes a provider string (twitter → x).
func ParseOAuthProviderID(raw string) (OAuthProviderID, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "google":
		return OAuthProviderGoogle, true
	case "facebook":
		return OAuthProviderFacebook, true
	case "github":
		return OAuthProviderGithub, true
	case "x", "twitter":
		return OAuthProviderX, true
	case "linkedin":
		return OAuthProviderLinkedin, true
	default:
		return "", false
	}
}

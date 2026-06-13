package models

// AuthenticationSettings holds per-project sign-in configuration (all project types).
type AuthenticationSettings struct {
	EnableGeneralAuth           *bool  `json:"enable_general_auth,omitempty" firestore:"enable_general_auth,omitempty" bson:"enable_general_auth,omitempty"`
	EnableGoogleAuth            *bool  `json:"enable_google_auth,omitempty" firestore:"enable_google_auth,omitempty" bson:"enable_google_auth,omitempty"`
	GeneralAuthenticationMethod string `json:"general_authentication_method,omitempty" firestore:"general_authentication_method,omitempty" bson:"general_authentication_method,omitempty"`
	GoogleClientID              string `json:"google_client_id,omitempty" firestore:"google_client_id,omitempty" bson:"google_client_id,omitempty"`
	GoogleClientSecret          string `json:"google_client_secret,omitempty" firestore:"google_client_secret,omitempty" bson:"google_client_secret,omitempty"`
	GoogleOAuthRedirectURI      string `json:"google_oauth_redirect_uri,omitempty" firestore:"google_oauth_redirect_uri,omitempty" bson:"google_oauth_redirect_uri,omitempty"`
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

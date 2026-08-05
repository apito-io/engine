package models

import "testing"

func TestApplyUpdateKeepsOAuthSecretWhenOmitted(t *testing.T) {
	p := &Project{
		AuthenticationSettings: &AuthenticationSettings{
			EnableFacebookAuth:       BoolPtr(true),
			FacebookClientID:         "fb-id",
			FacebookClientSecret:     "keep-me",
			FacebookOAuthRedirectURI: "https://app.example.com/cb",
		},
	}
	next, err := ApplyUpdateProjectAuthenticationInput(p, map[string]interface{}{
		"enable_facebook_auth":          true,
		"facebook_client_id":            "fb-id-2",
		"facebook_oauth_redirect_uri":   "https://app.example.com/cb2",
		"facebook_client_secret":        "",
	})
	if err != nil {
		t.Fatal(err)
	}
	if next.FacebookClientSecret != "keep-me" {
		t.Fatalf("expected secret kept, got %q", next.FacebookClientSecret)
	}
	if next.FacebookClientID != "fb-id-2" {
		t.Fatalf("expected client id updated, got %q", next.FacebookClientID)
	}
}

func TestOAuthCodeExchangeReady(t *testing.T) {
	p := &Project{
		AuthenticationSettings: &AuthenticationSettings{
			EnableGithubAuth:       BoolPtr(true),
			GithubClientID:         "gh",
			GithubClientSecret:     "sec",
			GithubOAuthRedirectURI: "https://app.example.com/gh",
		},
	}
	if !OAuthCodeExchangeReady(p, OAuthProviderGithub) {
		t.Fatal("expected github ready")
	}
	if OAuthCodeExchangeReady(p, OAuthProviderFacebook) {
		t.Fatal("facebook should not be ready")
	}
}

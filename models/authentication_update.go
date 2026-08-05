package models

import (
	"errors"
	"fmt"
	"strings"
)

// ApplyUpdateProjectAuthenticationInput merges GraphQL input into AuthenticationSettings.
func ApplyUpdateProjectAuthenticationInput(project *Project, input map[string]interface{}) (*AuthenticationSettings, error) {
	if project == nil {
		return nil, errors.New("project is nil")
	}
	next := AuthenticationSettings{}
	if project.AuthenticationSettings != nil {
		src := project.AuthenticationSettings
		next = *src
		// Re-copy bool pointers so we do not share with stored settings.
		if src.EnableGeneralAuth != nil {
			next.EnableGeneralAuth = BoolPtr(*src.EnableGeneralAuth)
		}
		if src.EnableGoogleAuth != nil {
			next.EnableGoogleAuth = BoolPtr(*src.EnableGoogleAuth)
		}
		if src.EnableFacebookAuth != nil {
			next.EnableFacebookAuth = BoolPtr(*src.EnableFacebookAuth)
		}
		if src.EnableGithubAuth != nil {
			next.EnableGithubAuth = BoolPtr(*src.EnableGithubAuth)
		}
		if src.EnableXAuth != nil {
			next.EnableXAuth = BoolPtr(*src.EnableXAuth)
		}
		if src.EnableLinkedinAuth != nil {
			next.EnableLinkedinAuth = BoolPtr(*src.EnableLinkedinAuth)
		}
	}
	if input == nil {
		return &next, nil
	}

	if err := applyBoolPtrField(input, "enable_general_auth", &next.EnableGeneralAuth); err != nil {
		return nil, err
	}
	if err := applyBoolPtrField(input, "enable_google_auth", &next.EnableGoogleAuth); err != nil {
		return nil, err
	}
	if err := applyBoolPtrField(input, "enable_facebook_auth", &next.EnableFacebookAuth); err != nil {
		return nil, err
	}
	if err := applyBoolPtrField(input, "enable_github_auth", &next.EnableGithubAuth); err != nil {
		return nil, err
	}
	if err := applyBoolPtrField(input, "enable_x_auth", &next.EnableXAuth); err != nil {
		return nil, err
	}
	if err := applyBoolPtrField(input, "enable_linkedin_auth", &next.EnableLinkedinAuth); err != nil {
		return nil, err
	}

	if v, ok := input["general_authentication_method"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("general_authentication_method: expected string")
		}
		switch strings.ToLower(strings.TrimSpace(s)) {
		case "email", "phone":
			next.GeneralAuthenticationMethod = strings.ToLower(strings.TrimSpace(s))
		default:
			return nil, errors.New("general_authentication_method must be email or phone")
		}
	}

	if err := applyTrimStringField(input, "google_client_id", &next.GoogleClientID); err != nil {
		return nil, err
	}
	if err := applySecretField(input, "google_client_secret", &next.GoogleClientSecret); err != nil {
		return nil, err
	}
	if err := applyOAuthRedirectField(input, "google_oauth_redirect_uri", &next.GoogleOAuthRedirectURI); err != nil {
		return nil, err
	}

	if err := applyTrimStringField(input, "facebook_client_id", &next.FacebookClientID); err != nil {
		return nil, err
	}
	if err := applySecretField(input, "facebook_client_secret", &next.FacebookClientSecret); err != nil {
		return nil, err
	}
	if err := applyOAuthRedirectField(input, "facebook_oauth_redirect_uri", &next.FacebookOAuthRedirectURI); err != nil {
		return nil, err
	}

	if err := applyTrimStringField(input, "github_client_id", &next.GithubClientID); err != nil {
		return nil, err
	}
	if err := applySecretField(input, "github_client_secret", &next.GithubClientSecret); err != nil {
		return nil, err
	}
	if err := applyOAuthRedirectField(input, "github_oauth_redirect_uri", &next.GithubOAuthRedirectURI); err != nil {
		return nil, err
	}

	if err := applyTrimStringField(input, "x_client_id", &next.XClientID); err != nil {
		return nil, err
	}
	if err := applySecretField(input, "x_client_secret", &next.XClientSecret); err != nil {
		return nil, err
	}
	if err := applyOAuthRedirectField(input, "x_oauth_redirect_uri", &next.XOAuthRedirectURI); err != nil {
		return nil, err
	}

	if err := applyTrimStringField(input, "linkedin_client_id", &next.LinkedinClientID); err != nil {
		return nil, err
	}
	if err := applySecretField(input, "linkedin_client_secret", &next.LinkedinClientSecret); err != nil {
		return nil, err
	}
	if err := applyOAuthRedirectField(input, "linkedin_oauth_redirect_uri", &next.LinkedinOAuthRedirectURI); err != nil {
		return nil, err
	}

	if _, has := input["default_registration_role"]; has {
		v := input["default_registration_role"]
		var s string
		if v != nil {
			var ok bool
			s, ok = v.(string)
			if !ok {
				return nil, fmt.Errorf("default_registration_role: expected string")
			}
		}
		s = strings.TrimSpace(s)
		if err := ValidateRegistrationDefaultRole(project, s); err != nil {
			return nil, err
		}
		next.DefaultRegistrationRole = s
	}
	return &next, nil
}

func applyBoolPtrField(input map[string]interface{}, key string, dest **bool) error {
	v, ok := input[key]
	if !ok || v == nil {
		return nil
	}
	b, ok := v.(bool)
	if !ok {
		return fmt.Errorf("%s: expected boolean", key)
	}
	*dest = BoolPtr(b)
	return nil
}

func applyTrimStringField(input map[string]interface{}, key string, dest *string) error {
	v, ok := input[key]
	if !ok || v == nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("%s: expected string", key)
	}
	*dest = strings.TrimSpace(s)
	return nil
}

func applySecretField(input map[string]interface{}, key string, dest *string) error {
	v, ok := input[key]
	if !ok || v == nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return fmt.Errorf("%s: expected string", key)
	}
	if t := strings.TrimSpace(s); t != "" {
		*dest = t
	}
	return nil
}

func applyOAuthRedirectField(input map[string]interface{}, key string, dest *string) error {
	if _, has := input[key]; !has {
		return nil
	}
	v := input[key]
	var s string
	if v != nil {
		var ok bool
		s, ok = v.(string)
		if !ok {
			return fmt.Errorf("%s: expected string", key)
		}
	}
	if err := ValidateOAuthRedirectURIForPersist(s, key); err != nil {
		return err
	}
	*dest = strings.TrimSpace(s)
	return nil
}

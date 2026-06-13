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
		next.GeneralAuthenticationMethod = src.GeneralAuthenticationMethod
		next.GoogleClientID = src.GoogleClientID
		next.GoogleClientSecret = src.GoogleClientSecret
		next.GoogleOAuthRedirectURI = src.GoogleOAuthRedirectURI
		next.DefaultRegistrationRole = src.DefaultRegistrationRole
		if src.EnableGeneralAuth != nil {
			next.EnableGeneralAuth = BoolPtr(*src.EnableGeneralAuth)
		}
		if src.EnableGoogleAuth != nil {
			next.EnableGoogleAuth = BoolPtr(*src.EnableGoogleAuth)
		}
	}
	if input == nil {
		return &next, nil
	}
	if v, ok := input["enable_general_auth"]; ok && v != nil {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("enable_general_auth: expected boolean")
		}
		next.EnableGeneralAuth = BoolPtr(b)
	}
	if v, ok := input["enable_google_auth"]; ok && v != nil {
		b, ok := v.(bool)
		if !ok {
			return nil, fmt.Errorf("enable_google_auth: expected boolean")
		}
		next.EnableGoogleAuth = BoolPtr(b)
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
	if v, ok := input["google_client_id"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("google_client_id: expected string")
		}
		next.GoogleClientID = strings.TrimSpace(s)
	}
	if v, ok := input["google_client_secret"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("google_client_secret: expected string")
		}
		if t := strings.TrimSpace(s); t != "" {
			next.GoogleClientSecret = t
		}
	}
	if _, has := input["google_oauth_redirect_uri"]; has {
		v := input["google_oauth_redirect_uri"]
		var s string
		if v != nil {
			var ok bool
			s, ok = v.(string)
			if !ok {
				return nil, fmt.Errorf("google_oauth_redirect_uri: expected string")
			}
		}
		if err := ValidateGoogleOAuthRedirectURIForPersist(s); err != nil {
			return nil, err
		}
		next.GoogleOAuthRedirectURI = strings.TrimSpace(s)
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

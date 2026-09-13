package services

import (
	"strings"

	"github.com/apito-io/engine/models"
)

const BootstrapKeyOnSecuredMessage = "BOOTSTRAP_KEY_ON_SECURED: use an app-user token"

func IsSecuredAppDataPath(path string) bool {
	return path == "/secured/graphql" ||
		path == "/secured/graphql/v2" ||
		strings.HasPrefix(path, "/secured/rest/") ||
		strings.HasPrefix(path, "/secured/files/") ||
		strings.HasPrefix(path, "/secured/upload/file")
}

func IsBootstrapLoginRole(role string) bool {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "login", "login_bootstrap":
		return true
	default:
		return false
	}
}

// IsStaffProjectUserKey reports whether an ak_ credential may call secured app data APIs.
// Website tenant tokens (TokenType tenant / IsProjectUser) remain allowed.
// Login bootstrap keys are rejected even if they carry a console operator UserID.
func IsStaffProjectUserKey(claims *models.TokenClaims) bool {
	if claims == nil {
		return false
	}
	if IsBootstrapLoginRole(claims.Role) {
		return false
	}
	if claims.IsProjectUser {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(claims.TokenType)) {
	case "user", "tenant":
		return true
	default:
		return false
	}
}

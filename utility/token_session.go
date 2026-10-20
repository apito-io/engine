package utility

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const issuedAtScopePrefix = "iat:"

// ProjectUserTokenLifetime is the maximum lifetime of a newly minted app-user token.
const ProjectUserTokenLifetime = 30 * 24 * time.Hour

// AppendIssuedAtScope records when a project-user token was minted.
// Existing iat scopes are preserved. The scope rides in the current token
// scope list so the encrypted payload format does not change.
func AppendIssuedAtScope(scopes []string, issuedAt int64) []string {
	if issuedAt <= 0 {
		return scopes
	}
	for _, scope := range scopes {
		if strings.HasPrefix(strings.TrimSpace(scope), issuedAtScopePrefix) {
			return scopes
		}
	}
	return append(scopes, fmt.Sprintf("%s%d", issuedAtScopePrefix, issuedAt))
}

// IssuedAtFromScopes returns the iat scope or zero when the token predates revocation.
func IssuedAtFromScopes(scopes []string) int64 {
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if !strings.HasPrefix(scope, issuedAtScopePrefix) {
			continue
		}
		n, err := strconv.ParseInt(strings.TrimPrefix(scope, issuedAtScopePrefix), 10, 64)
		if err != nil {
			return 0
		}
		return n
	}
	return 0
}

// ProjectUserSessionRevoked is true when a password reset or logout-all happened
// at or after the token was issued. Tokens without iat cannot prove they are newer.
func ProjectUserSessionRevoked(scopes []string, revokedAfter int64) bool {
	if revokedAfter <= 0 {
		return false
	}
	issued := IssuedAtFromScopes(scopes)
	return issued == 0 || issued <= revokedAfter
}

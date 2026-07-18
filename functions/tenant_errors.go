package functions

import "fmt"

// Stable tenant-scope error codes for function draft test and live callable.
const (
	TenantRequired       = "TENANT_REQUIRED"
	TenantNotFound       = "TENANT_NOT_FOUND"
	TenantNotActive      = "TENANT_NOT_ACTIVE"
	TenantScopeForbidden = "TENANT_SCOPE_FORBIDDEN"
	TenantDBNotReady     = "TENANT_DB_NOT_READY"
)

// TenantScopeError is returned before Deno/SQL when SaaS tenant scope is missing or invalid.
type TenantScopeError struct {
	Code    string
	Message string
}

func (e *TenantScopeError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message == "" {
		return e.Code
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func NewTenantScopeError(code, message string) *TenantScopeError {
	return &TenantScopeError{Code: code, Message: message}
}

// AsTenantScopeError returns the typed error when present.
func AsTenantScopeError(err error) (*TenantScopeError, bool) {
	if err == nil {
		return nil, false
	}
	if t, ok := err.(*TenantScopeError); ok {
		return t, true
	}
	return nil, false
}

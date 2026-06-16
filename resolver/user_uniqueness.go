package resolver

import (
	"errors"
	"strings"
)

// UserDuplicateExistsMessage returns a stable validation error for duplicate identity fields.
func UserDuplicateExistsMessage(field string, tenantScoped bool) string {
	if tenantScoped {
		return field + " already exists for this tenant"
	}
	return field + " already exists for this project"
}

// NormalizeUserEmail lowercases and trims an app user email for storage and lookup.
func NormalizeUserEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}

func (svc *ProjectUserService) assertUserEmailUnique(tenantID, emailLower, excludeUserID string) error {
	if emailLower == "" || svc == nil {
		return nil
	}
	rows, err := svc.ListByEmailWithFallback(tenantID, emailLower)
	if err != nil {
		return err
	}
	tenantScoped := strings.TrimSpace(tenantID) != ""
	for _, u := range rows {
		if u != nil && u.ID != excludeUserID {
			return errors.New(UserDuplicateExistsMessage("email", tenantScoped))
		}
	}
	return nil
}

func (svc *ProjectUserService) assertUserPhoneUnique(tenantID, phoneNorm, excludeUserID string) error {
	if phoneNorm == "" || svc == nil {
		return nil
	}
	rows, err := svc.ListByPhoneWithFallback(tenantID, phoneNorm)
	if err != nil {
		return err
	}
	tenantScoped := strings.TrimSpace(tenantID) != ""
	for _, u := range rows {
		if u != nil && u.ID != excludeUserID {
			return errors.New(UserDuplicateExistsMessage("phone", tenantScoped))
		}
	}
	return nil
}

// LinkTenantIDForUser returns the stored tenant_id for a user row when present.
func (svc *ProjectUserService) LinkTenantIDForUser(userID string, fallbackTenantID string) string {
	return svc.linkTenantIDForUser(userID, fallbackTenantID)
}

func (svc *ProjectUserService) linkTenantIDForUser(userID string, fallbackTenantID string) string {
	if svc == nil {
		return fallbackTenantID
	}
	row, err := svc.GetProjectAuthUserRow(userID)
	if err != nil || row == nil {
		return fallbackTenantID
	}
	if tid := strings.TrimSpace(row.TenantID); tid != "" {
		return tid
	}
	return fallbackTenantID
}

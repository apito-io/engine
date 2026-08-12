package resolver

import (
	"errors"
	"fmt"
	"strings"

	"github.com/apito-io/engine/models"
	"google.golang.org/api/idtoken"
)

// GoogleEmailVerified reports whether the Google ID token claims the email is verified.
func GoogleEmailVerified(payload *idtoken.Payload) bool {
	if payload == nil || payload.Claims == nil {
		return false
	}
	switch v := payload.Claims["email_verified"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

// GoogleEmailFromPayload extracts and normalizes email from a Google ID token payload.
func GoogleEmailFromPayload(payload *idtoken.Payload) string {
	if payload == nil || payload.Claims == nil {
		return ""
	}
	if v, ok := payload.Claims["email"].(string); ok {
		return NormalizeUserEmail(v)
	}
	return ""
}

// ResolveUserForGoogleLogin finds or creates an app user for Google login.
// googleSubLookupTenantID scopes google_sub lookup (SaaS login tenant when set).
// emailLookupTenantID scopes email lookup (empty = project-wide).
func (svc *ProjectUserService) ResolveUserForGoogleLogin(
	googleSub, email string,
	emailVerified bool,
	googleSubLookupTenantID, emailLookupTenantID string,
	createFn func() (*models.User, error),
) (*models.User, error) {
	if svc == nil {
		return nil, errors.New("project user service required")
	}
	sub := strings.TrimSpace(googleSub)
	if sub == "" {
		return nil, errors.New("google token missing subject")
	}

	candidates, err := svc.ListByGoogleSubWithFallback(googleSubLookupTenantID, sub)
	if err != nil {
		return nil, err
	}
	if len(candidates) > 1 {
		return nil, errors.New("multiple users matched this google subject")
	}
	if len(candidates) == 1 && candidates[0] != nil {
		return svc.backfillGoogleEmailIfEmpty(candidates[0], email, googleSubLookupTenantID)
	}

	if !emailVerified {
		return nil, errors.New("google email not verified")
	}

	emailLower := NormalizeUserEmail(email)
	if emailLower == "" {
		// Never create Google users without email (shared-DB SaaS orphans / 403s).
		return nil, errors.New("google token missing email")
	}

	byEmail, err := svc.ListByEmailWithFallback(emailLookupTenantID, emailLower)
	if err != nil {
		return nil, err
	}
	switch len(byEmail) {
	case 0:
		if createFn == nil {
			return nil, errors.New("google token missing email")
		}
		return createFn()
	case 1:
		existing := byEmail[0]
		if existing == nil {
			return nil, errors.New("user not found")
		}
		if existing.Status != models.UserStatusActive {
			return nil, errors.New("user is not active")
		}
		existingSub := strings.TrimSpace(existing.GoogleSub)
		if existingSub != "" && existingSub != sub {
			return nil, errors.New("google account already linked to another user")
		}
		existing.GoogleSub = sub
		linkTenant := svc.linkTenantIDForUser(existing.ID, emailLookupTenantID)
		if err := svc.UpdateUserRecord(existing, linkTenant); err != nil {
			return nil, fmt.Errorf("link google account: %w", err)
		}
		refreshed, err := svc.GetUserWithFallback(existing.ID, linkTenant)
		if err != nil {
			return nil, err
		}
		if refreshed == nil {
			return existing, nil
		}
		return refreshed, nil
	default:
		return nil, errors.New("multiple users matched this email")
	}
}

// backfillGoogleEmailIfEmpty updates a google_sub hit when the token has email but the row does not.
func (svc *ProjectUserService) backfillGoogleEmailIfEmpty(
	user *models.User,
	email, lookupTenantID string,
) (*models.User, error) {
	if user == nil {
		return nil, errors.New("user not found")
	}
	emailLower := NormalizeUserEmail(email)
	if emailLower == "" || strings.TrimSpace(user.Email) != "" {
		return user, nil
	}
	user.Email = emailLower
	linkTenant := svc.linkTenantIDForUser(user.ID, lookupTenantID)
	if err := svc.UpdateUserRecord(user, linkTenant); err != nil {
		return nil, fmt.Errorf("backfill google email: %w", err)
	}
	refreshed, err := svc.GetUserWithFallback(user.ID, linkTenant)
	if err != nil {
		return nil, err
	}
	if refreshed == nil {
		return user, nil
	}
	return refreshed, nil
}

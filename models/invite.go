package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	InviteStatusInvited  = "invited"
	InviteStatusAccepted = "accepted"
	InviteStatusExpired  = "expired"
)

// NewInviteToken returns a 64-char hex secret and its sha256 hex for storage.
func NewInviteToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	return raw, HashInviteToken(raw), nil
}

// HashInviteToken is sha256 hex of the raw accept-page token.
func HashInviteToken(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

// AcceptInviteURL is the public console page, e.g. https://app.apito.io/invite/accept?token=…
func AcceptInviteURL(origin, rawToken string) string {
	origin = strings.TrimRight(strings.TrimSpace(origin), "/")
	if origin == "" {
		origin = "http://localhost:4000"
	}
	if strings.TrimSpace(rawToken) == "" {
		return origin + "/auth/login"
	}
	return origin + "/invite/accept?token=" + url.QueryEscape(rawToken)
}

// EffectiveInviteStatus maps empty/legacy rows to accepted and past-due invited to expired.
func EffectiveInviteStatus(status, expiresAt string, now time.Time) string {
	s := strings.ToLower(strings.TrimSpace(status))
	if s == "" || s == InviteStatusAccepted {
		return InviteStatusAccepted
	}
	if s == InviteStatusInvited && InviteExpired(expiresAt, now) {
		return InviteStatusExpired
	}
	if s == InviteStatusExpired {
		return InviteStatusExpired
	}
	if s == InviteStatusInvited {
		return InviteStatusInvited
	}
	return InviteStatusAccepted
}

// InviteExpired is true when expiresAt is a past RFC3339 timestamp.
func InviteExpired(expiresAt string, now time.Time) bool {
	expiresAt = strings.TrimSpace(expiresAt)
	if expiresAt == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, expiresAt)
	if err != nil {
		return false
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return !t.After(now.UTC())
}

// GrantAllowsAccess is true for accepted (and legacy empty) grants.
func GrantAllowsAccess(status, expiresAt string, now time.Time) bool {
	return EffectiveInviteStatus(status, expiresAt, now) == InviteStatusAccepted
}

// ErrIfInviteBlocksAccess denies project switch / JWT until the invite is accepted.
func ErrIfInviteBlocksAccess(status, expiresAt string) error {
	switch EffectiveInviteStatus(status, expiresAt, time.Now().UTC()) {
	case InviteStatusInvited:
		return fmt.Errorf("invitation pending")
	case InviteStatusExpired:
		return fmt.Errorf("invitation expired")
	default:
		return nil
	}
}

// StampInviteOnRequest fills invite columns. Existing accepted grants stay accepted.
func StampInviteOnRequest(req *TeamMemberAddRequest, existing *UserProject, hash string, now time.Time, ttl time.Duration) {
	if req == nil {
		return
	}
	if now.IsZero() {
		now = time.Now().UTC()
	} else {
		now = now.UTC()
	}
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	req.InviteTokenHash = hash
	req.InvitedAt = now.Format(time.RFC3339)
	req.InviteExpiresAt = now.Add(ttl).Format(time.RFC3339)
	if existing != nil && GrantAllowsAccess(existing.InviteStatus, existing.InviteExpiresAt, now) {
		req.InviteStatus = InviteStatusAccepted
		if strings.TrimSpace(existing.AcceptedAt) != "" {
			req.AcceptedAt = existing.AcceptedAt
		} else {
			req.AcceptedAt = now.Format(time.RFC3339)
		}
		return
	}
	req.InviteStatus = InviteStatusInvited
}

// InviteExpireDuration reads INVITE_EXPIRE_DAYS (default 7).
func (c *Config) InviteExpireDuration() time.Duration {
	days := 7
	if c != nil && c.InviteExpireDays > 0 {
		days = c.InviteExpireDays
	}
	return time.Duration(days) * 24 * time.Hour
}

package models

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewInviteToken_HashRoundTrip(t *testing.T) {
	raw, hash, err := NewInviteToken()
	require.NoError(t, err)
	require.Len(t, raw, 64)
	require.Equal(t, HashInviteToken(raw), hash)
	require.NotEqual(t, raw, hash)
}

func TestEffectiveInviteStatus(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	require.Equal(t, InviteStatusAccepted, EffectiveInviteStatus("", "", now))
	require.Equal(t, InviteStatusAccepted, EffectiveInviteStatus("accepted", "", now))
	require.Equal(t, InviteStatusInvited, EffectiveInviteStatus("invited", now.Add(time.Hour).Format(time.RFC3339), now))
	require.Equal(t, InviteStatusExpired, EffectiveInviteStatus("invited", now.Add(-time.Hour).Format(time.RFC3339), now))
	require.True(t, GrantAllowsAccess("", "", now))
	require.False(t, GrantAllowsAccess("invited", now.Add(time.Hour).Format(time.RFC3339), now))
	require.Error(t, ErrIfInviteBlocksAccess("invited", now.Add(time.Hour).Format(time.RFC3339)))
	require.NoError(t, ErrIfInviteBlocksAccess("", ""))
}

func TestAcceptInviteURL(t *testing.T) {
	u := AcceptInviteURL("http://localhost:4000/", "abc def")
	require.True(t, strings.HasPrefix(u, "http://localhost:4000/invite/accept?token="))
	require.Contains(t, u, "abc")
}

func TestStampInviteOnRequest_KeepsAccepted(t *testing.T) {
	now := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	req := &TeamMemberAddRequest{UserID: "u1", ProjectID: "p1"}
	existing := &UserProject{InviteStatus: InviteStatusAccepted, AcceptedAt: "2026-01-01T00:00:00Z"}
	StampInviteOnRequest(req, existing, "hash", now, 7*24*time.Hour)
	require.Equal(t, InviteStatusAccepted, req.InviteStatus)
	require.Equal(t, "2026-01-01T00:00:00Z", req.AcceptedAt)
	require.Equal(t, "hash", req.InviteTokenHash)

	fresh := &TeamMemberAddRequest{UserID: "u1", ProjectID: "p2"}
	StampInviteOnRequest(fresh, nil, "hash", now, 7*24*time.Hour)
	require.Equal(t, InviteStatusInvited, fresh.InviteStatus)
	require.Equal(t, now.Add(7*24*time.Hour).Format(time.RFC3339), fresh.InviteExpiresAt)
}

func TestConfigInviteExpireDuration(t *testing.T) {
	require.Equal(t, 7*24*time.Hour, (*Config)(nil).InviteExpireDuration())
	require.Equal(t, 14*24*time.Hour, (&Config{InviteExpireDays: 14}).InviteExpireDuration())
}

package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeUserPhoneKey(t *testing.T) {
	require.Equal(t, "+15551234567", NormalizeUserPhoneKey("  +15551234567  "))
}

func TestUserToPublicMap(t *testing.T) {
	u := &User{ID: "u1", Email: "a@b.com", Role: "admin", Status: UserStatusActive}
	m := UserToPublicMap(u)
	require.Equal(t, "u1", m["id"])
	_, hasExt := m["ext"]
	require.False(t, hasExt)
}

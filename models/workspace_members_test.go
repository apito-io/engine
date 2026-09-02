package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMembershipPermissions_AliasesAndDrops(t *testing.T) {
	got := MembershipPermissions([]string{
		"content", "files", "api", "authentication", "teams", "addons", "extensions", "usages", "models",
	}, false)
	require.Equal(t, []string{"contents", "media", "api_explorer", "auth", "models"}, got)
}

func TestMembershipPermissions_AdminCopiesCatalog(t *testing.T) {
	got := MembershipPermissions(nil, true)
	require.Equal(t, GlobalPermissions, got)
	require.NotContains(t, got, "teams")
	require.Contains(t, got, "users")
	require.Contains(t, got, "database")
	require.Contains(t, got, "auth")
}

func TestCanonicalConsoleSection_UnknownEmpty(t *testing.T) {
	require.Equal(t, "", CanonicalConsoleSection("teams"))
	require.Equal(t, "webhook", CanonicalConsoleSection("webhooks"))
}

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

func TestNormalizeMembershipRole_OwnerPreserved(t *testing.T) {
	require.Equal(t, MembershipRoleOwner, NormalizeMembershipRole("owner", false))
	require.Equal(t, MembershipRoleAdmin, NormalizeMembershipRole("admin", false))
	require.Equal(t, MembershipRoleDeveloper, NormalizeMembershipRole("developer", false))
	require.Equal(t, MembershipRoleViewer, NormalizeMembershipRole("viewer", false))
	require.Equal(t, MembershipRoleTeam, NormalizeMembershipRole("editor", false))
	require.Equal(t, MembershipRoleAdmin, NormalizeMembershipRole("team", true))
}

func TestIsCollaboratorMembershipRole(t *testing.T) {
	require.True(t, IsCollaboratorMembershipRole("developer"))
	require.True(t, IsCollaboratorMembershipRole("viewer"))
	require.False(t, IsCollaboratorMembershipRole("owner"))
	require.False(t, IsCollaboratorMembershipRole("admin"))
}

func TestApplyProjectBillingLock(t *testing.T) {
	u := &SystemUser{}
	ApplyProjectBillingLock(u, &Project{TempBanned: true})
	require.True(t, u.ReadOnlyProject)
}

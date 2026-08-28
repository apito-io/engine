package services

import (
	"testing"

	"github.com/apito-io/engine/models"
	"github.com/golang-jwt/jwt"
	"github.com/stretchr/testify/require"
)

func TestStampPlatformAdminClaims_OnlyPersistedSuperAdmin(t *testing.T) {
	claims := jwt.MapClaims{}
	stampPlatformAdminClaims(claims, &models.SystemUser{IsAdmin: true, IsSuperAdmin: false})
	require.Equal(t, "false", claims["is_super_admin"])
	require.Equal(t, "true", claims["is_admin"])

	stampPlatformAdminClaims(claims, &models.SystemUser{IsAdmin: false, IsSuperAdmin: true})
	require.Equal(t, "true", claims["is_super_admin"])
}

func TestApplyIDTokenMapClaims_CopiesSuperAdmin(t *testing.T) {
	var tc models.TokenClaims
	err := applyIDTokenMapClaims(jwt.MapClaims{
		"account":        "user-1",
		"email":          "admin@apito.io",
		"is_super_admin": "true",
	}, &tc)
	require.NoError(t, err)
	require.True(t, tc.IsSuperAdmin)
	require.Equal(t, "user-1", tc.UserID)

	var tc2 models.TokenClaims
	err = applyIDTokenMapClaims(jwt.MapClaims{
		"account":        "user-2",
		"is_super_admin": true,
	}, &tc2)
	require.NoError(t, err)
	require.True(t, tc2.IsSuperAdmin)

	var tc3 models.TokenClaims
	err = applyIDTokenMapClaims(jwt.MapClaims{
		"account":        "user-3",
		"is_super_admin": "1",
	}, &tc3)
	require.NoError(t, err)
	require.True(t, tc3.IsSuperAdmin)

	var tc4 models.TokenClaims
	err = applyIDTokenMapClaims(jwt.MapClaims{
		"account": "user-4",
		"is_admin": "true",
	}, &tc4)
	require.NoError(t, err)
	require.False(t, tc4.IsSuperAdmin, "is_admin alone must not become is_super_admin")
}

func TestParseTruthyClaim(t *testing.T) {
	require.True(t, parseTruthyClaim("true"))
	require.True(t, parseTruthyClaim("TRUE"))
	require.True(t, parseTruthyClaim("1"))
	require.True(t, parseTruthyClaim(true))
	require.False(t, parseTruthyClaim("false"))
	require.False(t, parseTruthyClaim(nil))
}

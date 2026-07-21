package services

import (
	"strings"
	"testing"
	"time"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/models"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestParseRawToken(t *testing.T) {
	issuer, id, secret, err := ParseRawToken("apt_issuer1_token2_sec_ret")
	require.NoError(t, err)
	require.Equal(t, "issuer1", issuer)
	require.Equal(t, "token2", id)
	require.Equal(t, "sec_ret", secret)

	_, _, _, err = ParseRawToken("cli-legacy")
	require.Error(t, err)
}

func TestIsRetiredSyncTokenPrefix(t *testing.T) {
	require.True(t, IsRetiredSyncTokenPrefix("cli-abc"))
	require.True(t, IsRetiredSyncTokenPrefix("sdk-abc"))
	require.True(t, IsRetiredSyncTokenPrefix("mcp-abc"))
	require.False(t, IsRetiredSyncTokenPrefix("apt_x_y_z"))
	require.False(t, IsAccessToken("cli-abc"))
	require.True(t, IsAccessToken("apt_x_y_z"))
}

func TestIsAdministrableRole(t *testing.T) {
	require.True(t, IsAdministrableRole("admin"))
	require.True(t, IsAdministrableRole("Owner"))
	require.True(t, IsAdministrableRole("project_admin"))
	require.False(t, IsAdministrableRole("editor"))
}

func TestVerifyRecordExpiryAndRevoke(t *testing.T) {
	hash, err := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	require.NoError(t, err)
	rec := &models.AccessTokenRecord{
		Status:     models.AccessTokenStatusActive,
		SecretHash: string(hash),
		ExpiresAt:  time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02"),
	}
	require.NoError(t, verifyRecord(rec, "secret"))

	rec.Status = models.AccessTokenStatusRevoked
	require.Error(t, verifyRecord(rec, "secret"))

	rec.Status = models.AccessTokenStatusActive
	rec.ExpiresAt = time.Now().UTC().AddDate(0, 0, -2).Format("2006-01-02")
	require.Error(t, verifyRecord(rec, "secret"))
}

func TestAuthorizeTenant(t *testing.T) {
	svc := &AccessTokenService{}
	p := &models.AccessPrincipal{
		TenantGrantMode: models.AccessTokenTenantGrantSelected,
		TenantIDs:       map[string][]string{"proj1": {"tenA"}},
	}
	require.NoError(t, svc.AuthorizeTenant(p, "proj1", ""))
	require.NoError(t, svc.AuthorizeTenant(p, "proj1", "tenA"))
	require.Error(t, svc.AuthorizeTenant(p, "proj1", "tenB"))

	p.TenantGrantMode = models.AccessTokenTenantGrantNone
	require.Error(t, svc.AuthorizeTenant(p, "proj1", "tenA"))

	p.TenantGrantMode = models.AccessTokenTenantGrantAll
	require.NoError(t, svc.AuthorizeTenant(p, "proj1", "tenA"))
}

func TestCheckCIDRAllow(t *testing.T) {
	require.NoError(t, checkCIDRAllow(nil, "1.2.3.4"))
	require.NoError(t, checkCIDRAllow([]string{"1.2.3.4"}, "1.2.3.4"))
	require.Error(t, checkCIDRAllow([]string{"10.0.0.0/8"}, "1.2.3.4"))
	require.NoError(t, checkCIDRAllow([]string{"10.0.0.0/8"}, "10.1.2.3"))
}

func TestCapabilityImplies(t *testing.T) {
	caps, err := authz.ValidateCapabilities([]string{authz.CapSchemaPublish})
	require.NoError(t, err)
	require.True(t, authz.HasCapability(caps, authz.CapSchemaPublish))
	require.True(t, authz.HasCapability(caps, authz.CapSchemaWrite))
	require.True(t, authz.HasCapability(caps, authz.CapSchemaRead))
}

func TestParseTokenExpiry(t *testing.T) {
	s, err := parseTokenExpiry("2026-12-31", "")
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(s, "2026-12-31"))
	_, err = parseTokenExpiry("not-a-date", "")
	require.Error(t, err)
}

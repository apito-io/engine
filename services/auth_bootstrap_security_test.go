package services

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/apito-io/engine/authz"
	"github.com/apito-io/engine/models"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestMintSdkBootstrapVsCliSyncCapabilities(t *testing.T) {
	db := newMemDB()
	db.users["user1"] = &models.SystemUser{ID: "user1", Email: "a@b.com", IsActive: true}
	db.roles["user1|projA"] = "admin"
	svc := NewAccessTokenService(&models.Config{}, db)

	rawBoot, _, err := svc.Mint(context.Background(), "user1", &models.CreateAccessTokenRequest{
		Name:             "boot",
		Preset:           "sdk_bootstrap",
		ProjectGrantMode: models.AccessTokenProjectGrantSelected,
		ProjectIDs:       []string{"projA"},
	})
	require.NoError(t, err)
	_, principalBoot, err := svc.ValidateRaw(context.Background(), rawBoot, "", "")
	require.NoError(t, err)
	require.True(t, authz.HasCapability(principalBoot.Capabilities, authz.CapAuthLogin))
	require.True(t, authz.HasCapability(principalBoot.Capabilities, authz.CapAuthRegister))
	require.False(t, authz.HasCapability(principalBoot.Capabilities, authz.CapMembersWrite))

	rawSync, _, err := svc.Mint(context.Background(), "user1", &models.CreateAccessTokenRequest{
		Name:              "sync",
		Preset:            "cli_sync",
		ProjectGrantMode:  models.AccessTokenProjectGrantSelected,
		ProjectIDs:        []string{"projA"},
		AcknowledgeDanger: true,
	})
	require.NoError(t, err)
	_, principalSync, err := svc.ValidateRaw(context.Background(), rawSync, "", "")
	require.NoError(t, err)
	require.True(t, authz.HasCapability(principalSync.Capabilities, authz.CapSyncWrite))
	require.False(t, authz.HasCapability(principalSync.Capabilities, authz.CapAuthRegister))
	require.False(t, authz.HasCapability(principalSync.Capabilities, authz.CapAuthLogin))
}

func TestRequireCapability_AuthAndMembersGates(t *testing.T) {
	t.Setenv("APITO_ACCESS_POLICY_MODE", AccessPolicyEnforce)
	e := echo.New()
	withCaps := func(caps ...string) echo.Context {
		ctx := e.NewContext(httptest.NewRequest("POST", "/system/graphql", nil), httptest.NewRecorder())
		ctx.Set("access_principal", &models.AccessPrincipal{
			TokenID:      "t1",
			IssuerUserID: "u1",
			Capabilities: caps,
		})
		return ctx
	}

	// No apt_ principal (ak_ / cookie) → gates no-op.
	bare := e.NewContext(httptest.NewRequest("POST", "/system/graphql", nil), httptest.NewRecorder())
	require.NoError(t, RequireCapability(bare, authz.CapAuthLogin))
	require.NoError(t, RequireCapability(bare, authz.CapAuthRegister))
	require.NoError(t, RequireCapability(bare, authz.CapMembersWrite))

	boot := withCaps(authz.CapAuthLogin, authz.CapAuthRegister, authz.CapDataRead)
	require.NoError(t, RequireCapability(boot, authz.CapAuthLogin))
	require.NoError(t, RequireCapability(boot, authz.CapAuthRegister))
	err := RequireCapability(boot, authz.CapMembersWrite)
	require.Error(t, err)
	require.Contains(t, err.Error(), "CAPABILITY_DENIED")
	require.Contains(t, err.Error(), authz.CapMembersWrite)

	sync := withCaps(authz.CapSyncWrite, authz.CapSchemaWrite)
	err = RequireCapability(sync, authz.CapAuthRegister)
	require.Error(t, err)
	require.Contains(t, err.Error(), authz.CapAuthRegister)
	err = RequireCapability(sync, authz.CapAuthLogin)
	require.Error(t, err)
	require.Contains(t, err.Error(), authz.CapAuthLogin)
}

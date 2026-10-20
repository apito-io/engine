package authz

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegistryCompleteness(t *testing.T) {
	require.NotEmpty(t, All())
	require.NotEmpty(t, Presets())
	require.NotEmpty(t, DefaultOperationBindings())
	for _, b := range DefaultOperationBindings() {
		_, ok := Get(b.Capability)
		require.True(t, ok, "binding %s -> %s missing from registry", b.Operation, b.Capability)
	}
}

func TestResolvePreset(t *testing.T) {
	caps, err := ResolvePreset("read_only", nil)
	require.NoError(t, err)
	require.True(t, HasCapability(caps, CapProjectsRead))
	require.False(t, HasCapability(caps, CapDataWrite))

	_, err = ResolvePreset("nope", nil)
	require.Error(t, err)

	caps, err = ResolvePreset("custom", []string{CapDataRead})
	require.NoError(t, err)
	require.True(t, HasCapability(caps, CapDataRead))
}

func TestResolvePreset_SdkBootstrap(t *testing.T) {
	caps, err := ResolvePreset("sdk_bootstrap", nil)
	require.NoError(t, err)
	require.True(t, HasCapability(caps, CapProjectsRead))
	require.True(t, HasCapability(caps, CapAuthLogin))
	require.True(t, HasCapability(caps, CapAuthRegister))
	require.True(t, HasCapability(caps, CapSettingsRead))
	require.True(t, HasCapability(caps, CapDataRead))
	require.True(t, HasCapability(caps, CapPlansRead))
	require.False(t, HasCapability(caps, CapMembersWrite))
	require.False(t, HasCapability(caps, CapSchemaWrite))
	require.False(t, HasCapability(caps, CapSyncWrite))
}

func TestResolvePreset_MCPAssistant(t *testing.T) {
	caps, err := ResolvePreset("mcp_assistant", nil)
	require.NoError(t, err)
	require.True(t, HasCapability(caps, CapSchemaRead))
	require.True(t, HasCapability(caps, CapDataRead))
	require.False(t, HasCapability(caps, CapSchemaWrite))
	require.False(t, HasCapability(caps, CapSchemaPublish))
	require.False(t, HasCapability(caps, CapDatabaseRead))
	require.False(t, HasCapability(caps, CapDatabaseWrite))
}

func TestResolvePreset_MCPAdvanced(t *testing.T) {
	caps, err := ResolvePreset("mcp_advanced", nil)
	require.NoError(t, err)
	require.True(t, HasCapability(caps, CapSchemaWrite))
	require.True(t, HasCapability(caps, CapDatabaseRead))
	require.True(t, HasCapability(caps, CapDatabaseWrite))
	require.True(t, HasCapability(caps, CapTenantsWrite))
	require.False(t, HasCapability(caps, CapSchemaPublish))
	for _, preset := range Presets() {
		if preset.ID == "mcp_advanced" {
			require.True(t, preset.Danger)
			require.Equal(t, 30, preset.DefaultDays)
		}
	}
}

func TestResolvePreset_CLILogin(t *testing.T) {
	caps, err := ResolvePreset("cli_login", nil)
	require.NoError(t, err)
	require.True(t, HasCapability(caps, CapProjectsRead))
	require.True(t, HasCapability(caps, CapSchemaRead))
	require.False(t, HasCapability(caps, CapDataRead))
	require.False(t, HasCapability(caps, CapSchemaWrite))
	require.False(t, HasCapability(caps, CapSyncWrite))
	require.False(t, HasCapability(caps, CapFunctionsDeploy))
	require.False(t, HasCapability(caps, CapSettingsRead))
}

func TestValidateCapabilities_AuthCaps(t *testing.T) {
	caps, err := ValidateCapabilities([]string{CapAuthLogin, CapAuthRegister})
	require.NoError(t, err)
	require.True(t, HasCapability(caps, CapAuthLogin))
	require.True(t, HasCapability(caps, CapAuthRegister))

	_, err = ValidateCapabilities([]string{"auth.nope"})
	require.Error(t, err)
}

func TestDefaultOperationBindings_AuthOps(t *testing.T) {
	want := map[string]string{
		"loginUser":    CapAuthLogin,
		"registerUser": CapAuthRegister,
		"createUser":   CapMembersWrite,
	}
	found := map[string]string{}
	for _, b := range DefaultOperationBindings() {
		if _, ok := want[b.Operation]; ok {
			found[b.Operation] = b.Capability
		}
	}
	require.Equal(t, want, found)
}

func TestHasDangerCapability(t *testing.T) {
	require.True(t, HasDangerCapability([]string{CapSchemaPublish}))
	require.False(t, HasDangerCapability([]string{CapDataRead}))
}

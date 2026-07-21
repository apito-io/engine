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

func TestHasDangerCapability(t *testing.T) {
	require.True(t, HasDangerCapability([]string{CapSchemaPublish}))
	require.False(t, HasDangerCapability([]string{CapDataRead}))
}

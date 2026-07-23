package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildObjectKey(t *testing.T) {
	key, err := BuildObjectKey("proj-1", "", "media", "file-id", "png")
	require.NoError(t, err)
	require.Equal(t, "proj-1/media/file-id.png", key)
}

func TestBuildObjectKeyNoExt(t *testing.T) {
	key, err := BuildObjectKey("proj-1", "", "pdf", "file-id", "")
	require.NoError(t, err)
	require.Equal(t, "proj-1/pdf/file-id", key)
}

func TestBuildObjectKeyWithTenant(t *testing.T) {
	key, err := BuildObjectKey("proj-1", "01TENANT", "media", "file-id", "png")
	require.NoError(t, err)
	require.Equal(t, "proj-1/01TENANT/media/file-id.png", key)
}

func TestBuildObjectKeyRejectsUnsafeTenant(t *testing.T) {
	_, err := BuildObjectKey("proj-1", "../evil", "media", "file-id", "png")
	require.Error(t, err)
	_, err = BuildObjectKey("proj-1", "a/b", "media", "file-id", "png")
	require.Error(t, err)
}

func TestTenantObjectPrefix(t *testing.T) {
	prefix, err := TenantObjectPrefix("proj-1", "01TENANT")
	require.NoError(t, err)
	require.Equal(t, "proj-1/01TENANT/", prefix)
}

func TestTenantObjectPrefixRequiresIDs(t *testing.T) {
	_, err := TenantObjectPrefix("", "t1")
	require.Error(t, err)
	_, err = TenantObjectPrefix("proj-1", "")
	require.Error(t, err)
}

package storage

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildObjectKey(t *testing.T) {
	key := BuildObjectKey("proj-1", "media", "file-id", "png")
	require.Equal(t, "proj-1/media/file-id.png", key)
}

func TestBuildObjectKeyNoExt(t *testing.T) {
	key := BuildObjectKey("proj-1", "pdf", "file-id", "")
	require.Equal(t, "proj-1/pdf/file-id", key)
}

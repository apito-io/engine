package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSniffFileContentTypeAndExtensionPNG(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	ct, ext, ok := SniffFileContentTypeAndExtension(data)
	require.True(t, ok)
	require.Equal(t, "image/png", ct)
	require.Equal(t, "png", ext)
}

func TestSniffFileContentTypeAndExtensionPDF(t *testing.T) {
	data := []byte("%PDF-1.4\n")
	ct, ext, ok := SniffFileContentTypeAndExtension(data)
	require.True(t, ok)
	require.Equal(t, "application/pdf", ct)
	require.Equal(t, "pdf", ext)
}

func TestResolveUploadMIMEBlobPNG(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	ct, ext := ResolveUploadMIME("blob", "application/octet-stream", data)
	require.Equal(t, "image/png", ct)
	require.Equal(t, "png", ext)
}

func TestResolveUploadMIMEKeepsFilenameExtension(t *testing.T) {
	ct, ext := ResolveUploadMIME("report.pdf", "application/octet-stream", []byte("%PDF-1.4"))
	require.Equal(t, "pdf", ext)
	require.Equal(t, "application/pdf", ct)
}

func TestIsGenericUploadBaseName(t *testing.T) {
	require.True(t, IsGenericUploadBaseName("blob"))
	require.True(t, IsGenericUploadBaseName("upload"))
	require.False(t, IsGenericUploadBaseName("avatar"))
}

package models

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateFileType(t *testing.T) {
	require.NoError(t, ValidateFileType(SystemFileTypeMedia))
	require.NoError(t, ValidateFileType(SystemFileTypePDF))
	require.Error(t, ValidateFileType("invalid"))
}

func TestInferFileTypeFromMIME(t *testing.T) {
	require.Equal(t, SystemFileTypeMedia, InferFileTypeFromMIME("image/png"))
	require.Equal(t, SystemFileTypePDF, InferFileTypeFromMIME("application/pdf"))
	require.Equal(t, SystemFileTypeDocument, InferFileTypeFromMIME("text/plain"))
	require.Equal(t, SystemFileTypeOther, InferFileTypeFromMIME("application/octet-stream"))
}

func TestFileExtensionFromMIME(t *testing.T) {
	require.Equal(t, "png", FileExtensionFromMIME("image/png"))
	require.Equal(t, "jpg", FileExtensionFromMIME("image/jpeg"))
	require.Equal(t, "jpg", FileExtensionFromMIME("image/jpeg; charset=binary"))
	require.Equal(t, "pdf", FileExtensionFromMIME("application/pdf"))
	require.Equal(t, "", FileExtensionFromMIME("application/octet-stream"))
	require.Equal(t, "", FileExtensionFromMIME(""))
}

func TestSystemFileListParams(t *testing.T) {
	param := &CommonSystemParams{
		Ext: map[string]interface{}{
			ExtKeySystemFileType:    "media",
			ExtKeySystemFilesLimit:  10,
			ExtKeySystemFilesOffset: 5,
		},
	}
	ft, limit, offset := SystemFileListParams(param)
	require.Equal(t, "media", ft)
	require.Equal(t, 10, limit)
	require.Equal(t, 5, offset)
}

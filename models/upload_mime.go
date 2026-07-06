package models

import (
	"net/http"
	"path/filepath"
	"strings"

	svg "github.com/h2non/go-is-svg"
	"gopkg.in/h2non/filetype.v1"
)

// NormalizeContentType strips parameters and lowercases a MIME type value.
func NormalizeContentType(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return ct
}

// SniffFileContentTypeAndExtension detects MIME type and extension from file magic bytes.
func SniffFileContentTypeAndExtension(data []byte) (contentType, ext string, ok bool) {
	if len(data) == 0 {
		return "", "", false
	}
	if svg.Is(data) {
		return "image/svg+xml", "svg", true
	}
	kind, unknown := filetype.Match(data)
	if unknown == nil && kind.Extension != "" && kind.Extension != "unknown" {
		return kind.MIME.Value, kind.Extension, true
	}
	ct := http.DetectContentType(data)
	ct = NormalizeContentType(ct)
	if ct == "" || ct == "application/octet-stream" {
		return "", "", false
	}
	ext = FileExtensionFromMIME(ct)
	if ext == "" {
		return "", "", false
	}
	return ct, ext, true
}

// ResolveUploadMIME derives content type and extension from multipart metadata and file bytes.
// When the client sends a browser Blob (filename "blob", type application/octet-stream),
// magic-byte sniffing fills in the real format.
func ResolveUploadMIME(filename, headerContentType string, data []byte) (contentType, ext string) {
	contentType = NormalizeContentType(headerContentType)
	ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")

	genericCT := contentType == "" || contentType == "application/octet-stream"
	if ext == "" || genericCT {
		if sniffCT, sniffExt, ok := SniffFileContentTypeAndExtension(data); ok {
			if ext == "" {
				ext = sniffExt
			}
			if genericCT {
				contentType = sniffCT
			}
		}
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if ext == "" {
		ext = FileExtensionFromMIME(contentType)
	}
	return contentType, ext
}

// IsGenericUploadBaseName reports placeholder filenames from browser Blob uploads.
func IsGenericUploadBaseName(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "blob", "upload", "file", "unknown":
		return true
	default:
		return false
	}
}

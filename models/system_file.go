package models

import (
	"errors"
	"fmt"
	"mime"
	"strings"

)

const (
	SystemFileTypeMedia    = "media"
	SystemFileTypePDF      = "pdf"
	SystemFileTypeDocument = "document"
	SystemFileTypeOther    = "other"
)

const (
	ExtKeySystemFileType   = "file_type"
	ExtKeySystemFilesLimit = "files_limit"
	ExtKeySystemFilesOffset = "files_offset"
)

// ProjectFile is file metadata stored in the project database (table: files).
type ProjectFile struct {
	ORMBase `bun:"table:files,alias:pf"`

	ID            string `bun:"id,type:uuid,pk" json:"id,omitempty" firestore:"id,omitempty" bson:"_id,omitempty"`
	ProjectID     string `bun:"project_id,type:uuid,nullzero" json:"project_id,omitempty" firestore:"project_id,omitempty" bson:"project_id,omitempty"`
	FileType      string `bun:"file_type,notnull" json:"file_type,omitempty" firestore:"file_type,omitempty" bson:"file_type,omitempty"`
	FileName      string `bun:"file_name,notnull" json:"file_name,omitempty" firestore:"file_name,omitempty" bson:"file_name,omitempty"`
	FileExtension string `bun:"file_extension,nullzero" json:"file_extension,omitempty" firestore:"file_extension,omitempty" bson:"file_extension,omitempty"`
	ContentType   string `bun:"content_type,nullzero" json:"content_type,omitempty" firestore:"content_type,omitempty" bson:"content_type,omitempty"`
	Size          int64  `bun:"size,notnull" json:"size,omitempty" firestore:"size,omitempty" bson:"size,omitempty"`
	StorageKey    string `bun:"storage_key,notnull" json:"-" firestore:"storage_key,omitempty" bson:"storage_key,omitempty"`
	URL           string `bun:"url,nullzero" json:"url,omitempty" firestore:"url,omitempty" bson:"url,omitempty"`
	CreatedBy     string `bun:"created_by,type:uuid,nullzero" json:"created_by,omitempty" firestore:"created_by,omitempty" bson:"created_by,omitempty"`
	CreatedAt     string `bun:"created_at,nullzero" json:"created_at,omitempty" firestore:"created_at,omitempty" bson:"created_at,omitempty"`
	UpdatedAt     string `bun:"updated_at,nullzero" json:"updated_at,omitempty" firestore:"updated_at,omitempty" bson:"updated_at,omitempty"`
}

// SystemFile is an alias for ProjectFile (legacy name used by REST/SDK responses).
type SystemFile = ProjectFile

// ValidateFileType returns an error when s is not a supported file_type value.
func ValidateFileType(s string) error {
	switch strings.TrimSpace(s) {
	case SystemFileTypeMedia, SystemFileTypePDF, SystemFileTypeDocument, SystemFileTypeOther:
		return nil
	default:
		return fmt.Errorf("invalid file_type %q: must be one of media, pdf, document, other", s)
	}
}

// FileExtensionFromMIME returns a file extension without a leading dot (e.g. "png").
// Used when the upload filename has no extension (common for browser "blob" uploads).
func FileExtensionFromMIME(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	if ct == "" || ct == "application/octet-stream" {
		return ""
	}
	if ct == "image/jpeg" {
		return "jpg"
	}
	if exts, err := mime.ExtensionsByType(ct); err == nil && len(exts) > 0 {
		return strings.TrimPrefix(strings.ToLower(exts[0]), ".")
	}
	return fileExtensionFromMIMEFallback(ct)
}

func fileExtensionFromMIMEFallback(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return "jpg"
	case "image/png":
		return "png"
	case "image/gif":
		return "gif"
	case "image/webp":
		return "webp"
	case "image/svg+xml":
		return "svg"
	case "image/avif":
		return "avif"
	case "application/pdf":
		return "pdf"
	case "text/plain":
		return "txt"
	case "text/csv":
		return "csv"
	default:
		return ""
	}
}

// InferFileTypeFromMIME maps a MIME type to a system file_type category.
func InferFileTypeFromMIME(contentType string) string {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct == "" {
		return SystemFileTypeOther
	}
	if strings.HasPrefix(ct, "image/") {
		return SystemFileTypeMedia
	}
	if ct == "application/pdf" {
		return SystemFileTypePDF
	}
	if strings.HasPrefix(ct, "text/") ||
		strings.HasPrefix(ct, "application/msword") ||
		strings.HasPrefix(ct, "application/vnd.") ||
		strings.HasPrefix(ct, "application/json") ||
		strings.HasPrefix(ct, "application/rtf") {
		return SystemFileTypeDocument
	}
	return SystemFileTypeOther
}

// SystemFileListParams reads list filters from CommonSystemParams.Ext.
func SystemFileListParams(param *CommonSystemParams) (fileType string, limit, offset int) {
	limit = 50
	offset = 0
	if param == nil {
		return "", limit, offset
	}
	if param.Ext != nil {
		if v, ok := param.Ext[ExtKeySystemFileType].(string); ok {
			fileType = strings.TrimSpace(v)
		}
		if v, ok := param.Ext[ExtKeySystemFilesLimit].(int); ok && v > 0 {
			limit = v
		}
		if v, ok := param.Ext[ExtKeySystemFilesOffset].(int); ok && v >= 0 {
			offset = v
		}
	}
	return fileType, limit, offset
}

// ErrInvalidFileType is returned when file_type validation fails.
var ErrInvalidFileType = errors.New("invalid file_type")

package registry

import "fmt"

// Coded errors returned to Console/CLI. Stable strings; do not reword codes.
const (
	CodeRegistryUnavailable = "REGISTRY_UNAVAILABLE"
	CodeSignatureInvalid    = "SIGNATURE_INVALID"
	CodeIncompatibleEngine  = "INCOMPATIBLE_ENGINE"
	CodeUnsupportedPlatform = "UNSUPPORTED_PLATFORM"
	CodeChecksumMismatch    = "CHECKSUM_MISMATCH"
	CodeUnsafeArchive       = "UNSAFE_ARCHIVE"
	CodeConfigMismatch      = "CONFIG_MISMATCH"
	CodeHealthFailed        = "PLUGIN_HEALTH_FAILED"
	CodeRollbackCompleted   = "ROLLBACK_COMPLETED"
	CodeNotFound            = "PLUGIN_NOT_FOUND"
	CodeNotInstallable      = "PLUGIN_NOT_INSTALLABLE"
	CodeInUse               = "PLUGIN_IN_USE"
	CodeRegistryDisabled    = "REGISTRY_DISABLED"
	CodeDownloadFailed      = "DOWNLOAD_FAILED"
	CodeSizeMismatch        = "SIZE_MISMATCH"
	CodeHostNotAllowed      = "HOST_NOT_ALLOWED"
	CodeLocalUploadDisabled = "LOCAL_UPLOAD_DISABLED"
	CodeForbidden           = "FORBIDDEN"
)

// Error is a typed installer/catalog failure.
type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Cause }

func coded(code, msg string, cause error) *Error {
	return &Error{Code: code, Message: msg, Cause: cause}
}

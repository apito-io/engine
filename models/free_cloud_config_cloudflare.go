//go:build cloudflare

package models

import "strings"

// FreeCloudPlatformConfigured reports whether platform free-cloud storage is available.
// On Workers, a bound MEDIA_R2 bucket is sufficient (no AWS S3 secrets required).
func FreeCloudPlatformConfigured(cfg *Config) bool {
	if cfg != nil && cfg.CloudflareR2Bound {
		return true
	}
	if cfg == nil {
		return false
	}
	return strings.TrimSpace(cfg.FreeCloudDefaultS3AccessKey) != "" &&
		strings.TrimSpace(cfg.FreeCloudDefaultS3SecretKey) != "" &&
		strings.TrimSpace(cfg.FreeCloudDefaultS3Endpoint) != "" &&
		strings.TrimSpace(cfg.FreeCloudDefaultS3BucketName) != ""
}

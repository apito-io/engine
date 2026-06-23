//go:build !cloudflare

package models

import "strings"

// FreeCloudPlatformConfigured reports whether platform free-cloud S3/R2 credentials are set.
func FreeCloudPlatformConfigured(cfg *Config) bool {
	if cfg == nil {
		return false
	}
	return strings.TrimSpace(cfg.FreeCloudDefaultS3AccessKey) != "" &&
		strings.TrimSpace(cfg.FreeCloudDefaultS3SecretKey) != "" &&
		strings.TrimSpace(cfg.FreeCloudDefaultS3Endpoint) != "" &&
		strings.TrimSpace(cfg.FreeCloudDefaultS3BucketName) != ""
}

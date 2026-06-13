package models

import (
	"strings"
)

const defaultFreeCloudStorageLimitGB = 0.5

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

// FreeCloudStorageLimitBytes returns the per-project free-cloud quota in bytes.
func FreeCloudStorageLimitBytes(cfg *Config) int64 {
	limitGB := defaultFreeCloudStorageLimitGB
	if cfg != nil && cfg.FreeCloudStorageLimitGB > 0 {
		limitGB = cfg.FreeCloudStorageLimitGB
	}
	return int64(limitGB * 1024 * 1024 * 1024)
}

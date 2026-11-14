package models

import (
	"errors"
	"fmt"
	"strings"
)

// ProjectStorageRuntimeConfig is the resolved upload backend for a project.
type ProjectStorageRuntimeConfig struct {
	ProjectID       string
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	PublicBaseURL   string
	ForcePathStyle  bool
}

// ResolveProjectStorageConfig loads free-cloud platform creds or per-project custom storage.
func ResolveProjectStorageConfig(project *Project, cfg *Config) (*ProjectStorageRuntimeConfig, error) {
	if project == nil || strings.TrimSpace(project.ID) == "" {
		return nil, errors.New("project id required for storage config")
	}

	out := &ProjectStorageRuntimeConfig{ProjectID: project.ID}

	if UseFreeCloudStorageEffective(project) {
		if cfg == nil || !FreeCloudPlatformConfigured(cfg) {
			return nil, fmt.Errorf("platform free-cloud storage is not configured (FREE_CLOUD_DEFAULT_S3_*)")
		}
		out.Endpoint = strings.TrimSpace(cfg.FreeCloudDefaultS3Endpoint)
		out.Bucket = strings.TrimSpace(cfg.FreeCloudDefaultS3BucketName)
		out.AccessKeyID = strings.TrimSpace(cfg.FreeCloudDefaultS3AccessKey)
		out.SecretAccessKey = strings.TrimSpace(cfg.FreeCloudDefaultS3SecretKey)
		out.PublicBaseURL = freeCloudPublicBase(cfg)
		out.ForcePathStyle = cfg.FreeCloudDefaultS3ForcePathStyle
		return out, nil
	}

	if project.StorageSettings == nil {
		return nil, errors.New("custom storage is not configured")
	}
	st := project.StorageSettings
	out.Endpoint = strings.TrimSpace(st.Endpoint)
	out.Bucket = strings.TrimSpace(st.Bucket)
	out.AccessKeyID = strings.TrimSpace(st.AccessKeyID)
	out.SecretAccessKey = strings.TrimSpace(st.SecretAccessKey)
	out.Region = strings.TrimSpace(st.Region)
	out.PublicBaseURL = strings.TrimSpace(st.PublicBaseURL)
	if st.ForcePathStyle != nil {
		out.ForcePathStyle = *st.ForcePathStyle
	}

	if out.Endpoint == "" || out.Bucket == "" || out.AccessKeyID == "" || out.SecretAccessKey == "" {
		return nil, errors.New("custom storage is not fully configured")
	}
	return out, nil
}

// freeCloudPublicBase prefers the CDN host. The S3 public base is the fallback when the CDN URL is empty.
func freeCloudPublicBase(cfg *Config) string {
	if cfg == nil {
		return ""
	}
	if cdn := strings.TrimSpace(cfg.FreeCloudDefaultCDNURL); cdn != "" {
		return strings.TrimRight(cdn, "/")
	}
	return strings.TrimRight(strings.TrimSpace(cfg.FreeCloudDefaultS3PublicBaseURL), "/")
}

// PublicObjectURL joins a public base and an object key. The bucket name stays out of the path.
func PublicObjectURL(publicBase, key string) string {
	base := strings.TrimRight(strings.TrimSpace(publicBase), "/")
	objectKey := strings.TrimLeft(strings.TrimSpace(key), "/")
	if base == "" || objectKey == "" {
		return ""
	}
	return base + "/" + objectKey
}

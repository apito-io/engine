//go:build cloudflare

package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/apito-io/engine/models"
	"gitlab.com/apito.io/engine/cf_runtime"
)

// ProjectR2Storage uploads project files via Cloudflare R2 bindings (wired in cf_runtime).
type ProjectR2Storage struct {
	publicBaseURL string
}

// NewProjectS3Storage is the cloudflare entry point; R2 replaces native S3 SDK.
func NewProjectS3Storage(project *models.Project, cfg *models.Config) (*ProjectR2Storage, error) {
	if cfg == nil || !cfg.CloudflareR2Bound {
		return nil, fmt.Errorf("R2 project file storage: bind MEDIA_R2 in wrangler.toml")
	}
	base := strings.TrimRight(strings.TrimSpace(cf_runtime.GetEnv("MEDIA_PUBLIC_BASE_URL")), "/")
	if base == "" && cfg != nil {
		base = strings.TrimRight(strings.TrimSpace(cfg.FreeCloudDefaultS3PublicBaseURL), "/")
	}
	if base == "" && project != nil {
		base = strings.TrimRight(strings.TrimSpace(project.ID), "/")
	}
	return &ProjectR2Storage{publicBaseURL: base}, nil
}

func (s *ProjectR2Storage) PublicURL(key string) string {
	key = strings.TrimLeft(key, "/")
	if s.publicBaseURL == "" {
		return "/static/media/" + key
	}
	return s.publicBaseURL + "/" + key
}

func (s *ProjectR2Storage) Upload(ctx context.Context, key string, body io.Reader, contentType string, _ int64) (string, error) {
	_ = ctx
	if err := cf_runtime.R2Put(key, io.NopCloser(body), contentType); err != nil {
		return "", err
	}
	return s.PublicURL(key), nil
}

func (s *ProjectR2Storage) DeleteObjects(ctx context.Context, keys []string) ([]string, error) {
	_ = ctx
	var failed []string
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if err := cf_runtime.R2Delete(key); err != nil {
			failed = append(failed, key)
		}
	}
	if len(failed) > 0 {
		return failed, fmt.Errorf("failed to delete %d object(s) from R2", len(failed))
	}
	return nil, nil
}

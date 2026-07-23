package storage

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/apito-io/engine/models"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

const defaultAWSRegion = "us-east-1"

// ObjectStorage uploads and deletes objects in project-configured S3-compatible storage.
type ObjectStorage interface {
	Upload(ctx context.Context, key string, body io.Reader, contentType string, size int64) (publicURL string, err error)
	DeleteObjects(ctx context.Context, keys []string) (failed []string, err error)
	DeletePrefix(ctx context.Context, prefix string) (deleted int, failed []string, err error)
}

// ProjectS3Storage implements ObjectStorage using aws-sdk-go-v2 and project storage settings.
type ProjectS3Storage struct {
	client         *s3.Client
	bucket         string
	publicBaseURL  string
	forcePathStyle bool
}

// NewProjectS3Storage builds an S3-compatible client from project storage settings and platform free-cloud config.
func NewProjectS3Storage(project *models.Project, cfg *models.Config) (*ProjectS3Storage, error) {
	resolved, err := models.ResolveProjectStorageConfig(project, cfg)
	if err != nil {
		return nil, err
	}
	if resolved.AccessKeyID == "" || resolved.SecretAccessKey == "" {
		return nil, fmt.Errorf("storage credentials are not configured")
	}
	if resolved.Bucket == "" {
		return nil, fmt.Errorf("storage bucket is not configured")
	}

	region := resolved.Region
	if region == "" {
		region = defaultAWSRegion
	}

	loadOpts := []func(*aws.Config){
		func(o *aws.Config) {
			o.Region = region
			o.Credentials = credentials.NewStaticCredentialsProvider(
				resolved.AccessKeyID,
				resolved.SecretAccessKey,
				"",
			)
		},
	}
	if ep := strings.TrimSpace(resolved.Endpoint); ep != "" {
		loadOpts = append(loadOpts, func(o *aws.Config) {
			o.BaseEndpoint = aws.String(ep)
		})
	}

	awsCfg := aws.Config{}
	for _, opt := range loadOpts {
		opt(&awsCfg)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = resolved.ForcePathStyle
	})

	return &ProjectS3Storage{
		client:         client,
		bucket:         resolved.Bucket,
		publicBaseURL:  strings.TrimRight(resolved.PublicBaseURL, "/"),
		forcePathStyle: resolved.ForcePathStyle,
	}, nil
}

// ValidateTenantIDSegment returns a trimmed tenant id or an error when the value is unsafe for object keys.
func ValidateTenantIDSegment(tenantID string) (string, error) {
	tid := strings.TrimSpace(tenantID)
	if tid == "" {
		return "", nil
	}
	if strings.Contains(tid, "/") || strings.Contains(tid, "\\") || strings.Contains(tid, "..") {
		return "", fmt.Errorf("invalid tenant id for storage key")
	}
	return tid, nil
}

// BuildObjectKey returns the canonical storage key.
// General: {project_id}/{file_type}/{uuid}{ext}
// SaaS (non-empty tenantID): {project_id}/{tenant_id}/{file_type}/{uuid}{ext}
func BuildObjectKey(projectID, tenantID, fileType, fileID, ext string) (string, error) {
	tid, err := ValidateTenantIDSegment(tenantID)
	if err != nil {
		return "", err
	}
	ext = strings.TrimSpace(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	if tid != "" {
		return fmt.Sprintf("%s/%s/%s/%s%s", projectID, tid, fileType, fileID, ext), nil
	}
	return fmt.Sprintf("%s/%s/%s%s", projectID, fileType, fileID, ext), nil
}

// TenantObjectPrefix returns the S3 key prefix for all objects belonging to a SaaS tenant.
// Example: "protiva_bqyu3/01KXZ…/"
func TenantObjectPrefix(projectID, tenantID string) (string, error) {
	tid, err := ValidateTenantIDSegment(tenantID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(projectID) == "" || tid == "" {
		return "", fmt.Errorf("project id and tenant id are required for storage prefix")
	}
	return fmt.Sprintf("%s/%s/", strings.TrimSpace(projectID), tid), nil
}

// PublicURL builds the object URL from PublicBaseURL or endpoint/bucket/key.
func (s *ProjectS3Storage) PublicURL(key string) string {
	if s.publicBaseURL != "" {
		return s.publicBaseURL + "/" + strings.TrimLeft(key, "/")
	}
	if s.forcePathStyle && s.client.Options().BaseEndpoint != nil {
		base := strings.TrimRight(*s.client.Options().BaseEndpoint, "/")
		return fmt.Sprintf("%s/%s/%s", base, s.bucket, key)
	}
	ep := ""
	if s.client.Options().BaseEndpoint != nil {
		ep = strings.TrimSpace(*s.client.Options().BaseEndpoint)
	}
	if ep != "" {
		u, err := url.Parse(ep)
		if err == nil && u.Host != "" {
			return fmt.Sprintf("%s://%s.%s/%s", u.Scheme, s.bucket, u.Host, key)
		}
	}
	return fmt.Sprintf("https://%s.s3.amazonaws.com/%s", s.bucket, key)
}

// Upload streams an object to S3.
func (s *ProjectS3Storage) Upload(ctx context.Context, key string, body io.Reader, contentType string, size int64) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        body,
		ContentType: aws.String(contentType),
	}
	if size > 0 {
		input.ContentLength = aws.Int64(size)
	}
	_, err := s.client.PutObject(ctx, input)
	if err != nil {
		return "", err
	}
	return s.PublicURL(key), nil
}

// DeleteObjects removes objects; returns keys that failed to delete.
func (s *ProjectS3Storage) DeleteObjects(ctx context.Context, keys []string) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	var failed []string
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			continue
		}
		_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
			Bucket: aws.String(s.bucket),
			Key:    aws.String(key),
		})
		if err != nil {
			failed = append(failed, key)
		}
	}
	if len(failed) > 0 {
		return failed, fmt.Errorf("failed to delete %d object(s) from storage", len(failed))
	}
	return nil, nil
}

// DeletePrefix lists all objects under prefix and deletes them.
func (s *ProjectS3Storage) DeletePrefix(ctx context.Context, prefix string) (int, []string, error) {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return 0, nil, fmt.Errorf("prefix is required")
	}

	var keys []string
	var token *string
	for {
		out, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(s.bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return 0, nil, err
		}
		for _, obj := range out.Contents {
			if obj.Key == nil {
				continue
			}
			key := strings.TrimSpace(*obj.Key)
			if key == "" {
				continue
			}
			keys = append(keys, key)
		}
		if !aws.ToBool(out.IsTruncated) {
			break
		}
		token = out.NextContinuationToken
	}

	if len(keys) == 0 {
		return 0, nil, nil
	}

	failed, err := s.DeleteObjects(ctx, keys)
	deleted := len(keys) - len(failed)
	return deleted, failed, err
}

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
}

// ProjectS3Storage implements ObjectStorage using aws-sdk-go-v2 and project storage settings.
type ProjectS3Storage struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
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

// BuildObjectKey returns the canonical storage key: {project_id}/{file_type}/{uuid}{ext}.
func BuildObjectKey(projectID, fileType, fileID, ext string) string {
	ext = strings.TrimSpace(ext)
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return fmt.Sprintf("%s/%s/%s%s", projectID, fileType, fileID, ext)
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

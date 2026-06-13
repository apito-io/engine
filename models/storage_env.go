package models

import (
	"strconv"

	"github.com/apito-io/types/protobuff"
)

const (
	storageEnvProjectID          = "PROJECT_ID"
	storageEnvEndpoint           = "STORAGE_ENDPOINT"
	storageEnvBucket             = "STORAGE_BUCKET"
	storageEnvAccessKeyID        = "STORAGE_ACCESS_KEY_ID"
	storageEnvSecretAccessKey    = "STORAGE_SECRET_ACCESS_KEY"
	storageEnvRegion             = "STORAGE_REGION"
	storageEnvPublicBaseURL      = "STORAGE_PUBLIC_BASE_URL"
	storageEnvForcePathStyle     = "STORAGE_FORCE_PATH_STYLE"
)

// BuildStoragePluginEnvVars returns env vars for storage plugin Init from resolved project storage.
func BuildStoragePluginEnvVars(project *Project, cfg *Config) ([]*protobuff.EnvVariable, error) {
	resolved, err := ResolveProjectStorageConfig(project, cfg)
	if err != nil {
		return nil, err
	}

	envs := []*protobuff.EnvVariable{
		{Key: storageEnvProjectID, Value: resolved.ProjectID},
		{Key: storageEnvEndpoint, Value: resolved.Endpoint},
		{Key: storageEnvBucket, Value: resolved.Bucket},
		{Key: storageEnvAccessKeyID, Value: resolved.AccessKeyID},
		{Key: storageEnvSecretAccessKey, Value: resolved.SecretAccessKey},
	}
	if resolved.Region != "" {
		envs = append(envs, &protobuff.EnvVariable{Key: storageEnvRegion, Value: resolved.Region})
	}
	if resolved.PublicBaseURL != "" {
		envs = append(envs, &protobuff.EnvVariable{Key: storageEnvPublicBaseURL, Value: resolved.PublicBaseURL})
	}
	if resolved.ForcePathStyle {
		envs = append(envs, &protobuff.EnvVariable{Key: storageEnvForcePathStyle, Value: strconv.FormatBool(true)})
	}
	return envs, nil
}

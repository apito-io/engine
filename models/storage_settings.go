package models

import (
	"errors"
	"strings"
)

// StorageSettings configures project media/object storage (Apito-hosted or custom S3-compatible).
type StorageSettings struct {
	UseFreeCloudStorage *bool  `json:"use_free_cloud_storage,omitempty" firestore:"use_free_cloud_storage,omitempty" bson:"use_free_cloud_storage,omitempty"`
	Endpoint            string `json:"endpoint,omitempty" firestore:"endpoint,omitempty" bson:"endpoint,omitempty"`
	Region              string `json:"region,omitempty" firestore:"region,omitempty" bson:"region,omitempty"`
	Bucket              string `json:"bucket,omitempty" firestore:"bucket,omitempty" bson:"bucket,omitempty"`
	AccessKeyID         string `json:"access_key_id,omitempty" firestore:"access_key_id,omitempty" bson:"access_key_id,omitempty"`
	SecretAccessKey     string `json:"secret_access_key,omitempty" firestore:"secret_access_key,omitempty" bson:"secret_access_key,omitempty"`
	PublicBaseURL       string `json:"public_base_url,omitempty" firestore:"public_base_url,omitempty" bson:"public_base_url,omitempty"`
	ForcePathStyle      *bool  `json:"force_path_style,omitempty" firestore:"force_path_style,omitempty" bson:"force_path_style,omitempty"`
}

// UseFreeCloudStorageEffective reports whether Apito platform storage should be used.
func UseFreeCloudStorageEffective(p *Project) bool {
	if p == nil || p.StorageSettings == nil || p.StorageSettings.UseFreeCloudStorage == nil {
		return false
	}
	return *p.StorageSettings.UseFreeCloudStorage
}

// HasSecretAccessKeyConfigured reports whether a non-empty secret access key is stored.
func HasSecretAccessKeyConfigured(p *Project) bool {
	if p == nil || p.StorageSettings == nil {
		return false
	}
	return strings.TrimSpace(p.StorageSettings.SecretAccessKey) != ""
}

// ProjectStorageConfigured reports whether uploads can proceed for the project's storage mode.
func ProjectStorageConfigured(p *Project, cfg *Config) bool {
	if p == nil {
		return false
	}
	if UseFreeCloudStorageEffective(p) {
		return FreeCloudPlatformConfigured(cfg)
	}
	if p.StorageSettings == nil {
		return false
	}
	st := p.StorageSettings
	return strings.TrimSpace(st.Endpoint) != "" &&
		strings.TrimSpace(st.Bucket) != "" &&
		strings.TrimSpace(st.AccessKeyID) != "" &&
		HasSecretAccessKeyConfigured(p)
}

// ApplyUpdateProjectStorageInput merges GraphQL/storage update input into StorageSettings.
func ApplyUpdateProjectStorageInput(existing *Project, input map[string]interface{}, hasExistingSecret bool) (*StorageSettings, error) {
	next := StorageSettings{}
	if existing != nil && existing.StorageSettings != nil {
		src := existing.StorageSettings
		next.UseFreeCloudStorage = src.UseFreeCloudStorage
		next.Endpoint = src.Endpoint
		next.Region = src.Region
		next.Bucket = src.Bucket
		next.AccessKeyID = src.AccessKeyID
		next.SecretAccessKey = src.SecretAccessKey
		next.PublicBaseURL = src.PublicBaseURL
		next.ForcePathStyle = src.ForcePathStyle
	}
	if input == nil {
		return &next, nil
	}

	useFree := false
	if v, ok := input["use_free_cloud_storage"]; ok && v != nil {
		b, ok := v.(bool)
		if !ok {
			return nil, errors.New("use_free_cloud_storage: expected boolean")
		}
		useFree = b
		next.UseFreeCloudStorage = BoolPtr(b)
	}

	if useFree {
		if storageInputHasCustomFields(input) {
			return nil, errors.New("custom storage fields cannot be set when use_free_cloud_storage is enabled")
		}
		next.Endpoint = ""
		next.Region = ""
		next.Bucket = ""
		next.AccessKeyID = ""
		next.SecretAccessKey = ""
		next.PublicBaseURL = ""
		next.ForcePathStyle = nil
		return &next, nil
	}

	if v, ok := input["endpoint"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("endpoint: expected string")
		}
		next.Endpoint = strings.TrimSpace(s)
	}
	if v, ok := input["region"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("region: expected string")
		}
		next.Region = strings.TrimSpace(s)
	}
	if v, ok := input["bucket"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("bucket: expected string")
		}
		next.Bucket = strings.TrimSpace(s)
	}
	if v, ok := input["access_key_id"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("access_key_id: expected string")
		}
		next.AccessKeyID = strings.TrimSpace(s)
	}
	if v, ok := input["secret_access_key"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("secret_access_key: expected string")
		}
		if t := strings.TrimSpace(s); t != "" {
			next.SecretAccessKey = t
		}
	}
	if v, ok := input["public_base_url"]; ok && v != nil {
		s, ok := v.(string)
		if !ok {
			return nil, errors.New("public_base_url: expected string")
		}
		next.PublicBaseURL = strings.TrimSpace(s)
	}
	if v, ok := input["force_path_style"]; ok && v != nil {
		b, ok := v.(bool)
		if !ok {
			return nil, errors.New("force_path_style: expected boolean")
		}
		next.ForcePathStyle = BoolPtr(b)
	}

	if strings.TrimSpace(next.Endpoint) == "" {
		return nil, errors.New("endpoint is required for custom storage")
	}
	if strings.TrimSpace(next.Bucket) == "" {
		return nil, errors.New("bucket is required for custom storage")
	}
	if strings.TrimSpace(next.AccessKeyID) == "" {
		return nil, errors.New("access_key_id is required for custom storage")
	}
	if strings.TrimSpace(next.SecretAccessKey) == "" && !hasExistingSecret {
		return nil, errors.New("secret_access_key is required for custom storage")
	}
	return &next, nil
}

func storageInputHasCustomFields(input map[string]interface{}) bool {
	check := func(key string) bool {
		v, ok := input[key]
		if !ok || v == nil {
			return false
		}
		switch t := v.(type) {
		case string:
			return strings.TrimSpace(t) != ""
		case bool:
			return t
		default:
			return true
		}
	}
	return check("endpoint") || check("region") || check("bucket") ||
		check("access_key_id") || check("secret_access_key") || check("public_base_url") || check("force_path_style")
}

package models

import "testing"

func TestResolveProjectStorageConfigFreeCloud(t *testing.T) {
	p := &Project{
		ID: "proj-1",
		StorageSettings: &StorageSettings{
			UseFreeCloudStorage: BoolPtr(true),
		},
	}
	cfg := &Config{
		FreeCloudDefaultS3AccessKey:  "key",
		FreeCloudDefaultS3SecretKey:  "secret",
		FreeCloudDefaultS3Endpoint:     "https://r2.example.com",
		FreeCloudDefaultS3BucketName:   "bucket",
		FreeCloudDefaultS3ForcePathStyle: true,
	}
	got, err := ResolveProjectStorageConfig(p, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.Endpoint != "https://r2.example.com" || got.Bucket != "bucket" || !got.ForcePathStyle {
		t.Fatalf("unexpected free cloud config: %+v", got)
	}
}

func TestResolveProjectStorageConfigCustom(t *testing.T) {
	p := &Project{
		ID: "proj-1",
		StorageSettings: &StorageSettings{
			UseFreeCloudStorage: BoolPtr(false),
			Endpoint:            "https://minio.example.com",
			Bucket:              "uploads",
			AccessKeyID:         "ak",
			SecretAccessKey:     "sk",
		},
	}
	got, err := ResolveProjectStorageConfig(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Endpoint != "https://minio.example.com" || got.AccessKeyID != "ak" {
		t.Fatalf("unexpected custom config: %+v", got)
	}
}

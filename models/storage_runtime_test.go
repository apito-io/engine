package models

import (
	"strings"
	"testing"
)

func TestResolveProjectStorageConfigFreeCloud(t *testing.T) {
	p := &Project{
		ID: "proj-1",
		StorageSettings: &StorageSettings{
			UseFreeCloudStorage: BoolPtr(true),
		},
	}
	cfg := &Config{
		FreeCloudDefaultS3AccessKey:      "key",
		FreeCloudDefaultS3SecretKey:      "secret",
		FreeCloudDefaultS3Endpoint:       "https://r2.example.com",
		FreeCloudDefaultS3BucketName:     "bucket",
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

func TestResolveProjectStorageConfigFreeCloudCDN(t *testing.T) {
	p := &Project{
		ID: "apito_website",
		StorageSettings: &StorageSettings{
			UseFreeCloudStorage: BoolPtr(true),
		},
	}
	cfg := &Config{
		FreeCloudDefaultS3AccessKey:      "key",
		FreeCloudDefaultS3SecretKey:      "secret",
		FreeCloudDefaultS3Endpoint:       "https://account.r2.cloudflarestorage.com",
		FreeCloudDefaultS3BucketName:     "apito-free-storage",
		FreeCloudDefaultS3PublicBaseURL:  "https://files.example",
		FreeCloudDefaultCDNURL:           "https://cdn.apito.io/",
		FreeCloudDefaultS3ForcePathStyle: true,
	}
	got, err := ResolveProjectStorageConfig(p, cfg)
	if err != nil {
		t.Fatal(err)
	}
	const key = "apito_website/media/01M7B9ZGZM0Q1759ZHYE8WK3X3.jpg"
	url := PublicObjectURL(got.PublicBaseURL, key)
	const want = "https://cdn.apito.io/apito_website/media/01M7B9ZGZM0Q1759ZHYE8WK3X3.jpg"
	if url != want {
		t.Fatalf("url = %s", url)
	}
	if strings.Contains(url, ".r2.cloudflarestorage.com") || strings.Contains(url, "apito-free-storage") {
		t.Fatalf("cdn url still contains the storage host or bucket: %s", url)
	}
}

func TestResolveProjectStorageConfigFreeCloudPublicBaseFallback(t *testing.T) {
	p := &Project{
		ID: "proj-1",
		StorageSettings: &StorageSettings{
			UseFreeCloudStorage: BoolPtr(true),
		},
	}
	cfg := &Config{
		FreeCloudDefaultS3AccessKey:     "key",
		FreeCloudDefaultS3SecretKey:     "secret",
		FreeCloudDefaultS3Endpoint:      "https://r2.example.com",
		FreeCloudDefaultS3BucketName:    "bucket",
		FreeCloudDefaultS3PublicBaseURL: "https://files.example/",
	}
	got, err := ResolveProjectStorageConfig(p, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicBaseURL != "https://files.example" {
		t.Fatalf("public base = %s", got.PublicBaseURL)
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

package models

import "testing"

func TestApplyUpdateProjectStorageInputFreeRejectsCustomFields(t *testing.T) {
	_, err := ApplyUpdateProjectStorageInput(&Project{}, map[string]interface{}{
		"use_free_cloud_storage": true,
		"endpoint":               "https://s3.example.com",
	}, false)
	if err == nil {
		t.Fatal("expected mutual exclusion error")
	}
}

func TestApplyUpdateProjectStorageInputCustomRequiresFields(t *testing.T) {
	_, err := ApplyUpdateProjectStorageInput(&Project{}, map[string]interface{}{
		"use_free_cloud_storage": false,
		"endpoint":               "https://s3.example.com",
		"bucket":                 "b",
		"access_key_id":          "key",
		"secret_access_key":      "sec",
	}, false)
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
}

func TestApplyUpdateProjectStorageInputFreeClearsCustom(t *testing.T) {
	p := &Project{
		StorageSettings: &StorageSettings{
			Endpoint:        "https://old.example.com",
			Bucket:          "old",
			AccessKeyID:     "old",
			SecretAccessKey: "old",
		},
	}
	next, err := ApplyUpdateProjectStorageInput(p, map[string]interface{}{
		"use_free_cloud_storage": true,
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if next.Endpoint != "" || next.Bucket != "" {
		t.Fatalf("expected cleared custom fields, got %+v", next)
	}
	if next.UseFreeCloudStorage == nil || !*next.UseFreeCloudStorage {
		t.Fatal("expected use_free_cloud_storage true")
	}
}

func TestProjectStorageConfiguredCustom(t *testing.T) {
	p := &Project{
		StorageSettings: &StorageSettings{
			Endpoint:        "https://s3.example.com",
			Bucket:          "b",
			AccessKeyID:     "k",
			SecretAccessKey: "s",
		},
	}
	if !ProjectStorageConfigured(p, nil) {
		t.Fatal("expected custom configured")
	}
}

func TestProjectStorageConfiguredFreeCloud(t *testing.T) {
	p := &Project{
		StorageSettings: &StorageSettings{
			UseFreeCloudStorage: BoolPtr(true),
		},
	}
	cfg := &Config{
		FreeCloudDefaultS3AccessKey:  "key",
		FreeCloudDefaultS3SecretKey:  "secret",
		FreeCloudDefaultS3Endpoint:     "https://r2.example.com",
		FreeCloudDefaultS3BucketName:   "bucket",
	}
	if !ProjectStorageConfigured(p, cfg) {
		t.Fatal("expected free cloud configured")
	}
}

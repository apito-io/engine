package models

import "testing"

func TestFreeCloudStorageLimitBytes(t *testing.T) {
	if got := FreeCloudStorageLimitBytes(nil); got != int64(0.5*1024*1024*1024) {
		t.Fatalf("expected default 0.5GB, got %d", got)
	}
	cfg := &Config{FreeCloudStorageLimitGB: 1}
	if got := FreeCloudStorageLimitBytes(cfg); got != 1*1024*1024*1024 {
		t.Fatalf("expected 1GB, got %d", got)
	}
}

package resolver

import (
	"context"
	"sync"
	"testing"

	"gitlab.com/apito.io/open_driver/cache/memory"
	"github.com/apito-io/engine/models"
)

type storageSettingsSystemDriver struct {
	trackingSystemDriver
	mu      sync.Mutex
	project *models.Project
}

func (d *storageSettingsSystemDriver) SaveProjectStorageSettings(_ context.Context, projectID string, storage *models.StorageSettings) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.project == nil || d.project.ID != projectID {
		return nil
	}
	d.project.StorageSettings = storage
	return nil
}

func (d *storageSettingsSystemDriver) GetProject(_ context.Context, projectID string) (*models.Project, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.project == nil || d.project.ID != projectID {
		return nil, nil
	}
	copy := *d.project
	if d.project.StorageSettings != nil {
		st := *d.project.StorageSettings
		copy.StorageSettings = &st
	}
	return &copy, nil
}

func (d *storageSettingsSystemDriver) storageBucket() string {
	if d.project == nil || d.project.StorageSettings == nil {
		return ""
	}
	return d.project.StorageSettings.Bucket
}

func TestLoadProjectCacheReturnsStaleStorageBucketWithoutRefresh(t *testing.T) {
	ctx := context.Background()
	cacheDriver, err := memory.GetMemoryCacheDriver(&models.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sys := &storageSettingsSystemDriver{
		project: &models.Project{
			ID: "p1",
			StorageSettings: &models.StorageSettings{
				Bucket:   "old-bucket",
				Endpoint: "https://old.example",
			},
		},
	}
	srv := &GraphQLServer{
		Cfg:          &models.Config{},
		SystemDriver: sys,
		ProjectCache: cacheDriver,
	}

	seed := *sys.project
	if sys.project.StorageSettings != nil {
		st := *sys.project.StorageSettings
		seed.StorageSettings = &st
	}
	if _, err := cacheDriver.SaveProject(ctx, &seed); err != nil {
		t.Fatal(err)
	}

	if err := sys.SaveProjectStorageSettings(ctx, "p1", &models.StorageSettings{
		Bucket:   "new-bucket",
		Endpoint: "https://new.example",
	}); err != nil {
		t.Fatal(err)
	}

	loaded, err := srv.LoadProjectCache(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.StorageSettings.Bucket; got != "old-bucket" {
		t.Fatalf("without cache refresh, bucket = %q, want stale old-bucket", got)
	}
	if sys.storageBucket() != "new-bucket" {
		t.Fatalf("system DB bucket = %q, want new-bucket", sys.storageBucket())
	}
}

func TestRefreshProjectAndReCachePicksUpStorageSettings(t *testing.T) {
	ctx := context.Background()
	cacheDriver, err := memory.GetMemoryCacheDriver(&models.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sys := &storageSettingsSystemDriver{
		project: &models.Project{
			ID: "p1",
			StorageSettings: &models.StorageSettings{
				Bucket:   "old-bucket",
				Endpoint: "https://old.example",
			},
		},
	}
	srv := &GraphQLServer{
		Cfg:          &models.Config{},
		SystemDriver: sys,
		ProjectCache: cacheDriver,
	}

	seed := *sys.project
	if sys.project.StorageSettings != nil {
		st := *sys.project.StorageSettings
		seed.StorageSettings = &st
	}
	if _, err := cacheDriver.SaveProject(ctx, &seed); err != nil {
		t.Fatal(err)
	}
	if err := sys.SaveProjectStorageSettings(ctx, "p1", &models.StorageSettings{
		Bucket:   "new-bucket",
		Endpoint: "https://new.example",
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := srv.refreshProjectAndReCache(ctx, "p1"); err != nil {
		t.Fatal(err)
	}
	loaded, err := srv.LoadProjectCache(ctx, "p1")
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.StorageSettings.Bucket; got != "new-bucket" {
		t.Fatalf("after refresh, bucket = %q, want new-bucket", got)
	}
}

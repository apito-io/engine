package bbolt

import (
	"context"
	"errors"
	"fmt"
	"time"

	apitobolt "github.com/apito-io/apitoBolt"
	"github.com/apito-io/engine/models"
)

// CacheItem represents a generic cache item
type CacheItem struct {
	ID   string      `json:"id"`
	Data interface{} `json:"data"`
}

// Put stores a value in the cache with optional TTL
func (c *CacheDriver) Put(ctx context.Context, id string, cache interface{}) error {
	ttl := c.parseTTL()
	return c.putWithTTL(ctx, "cache", id, cache, ttl)
}

// Get retrieves a value from the cache
func (c *CacheDriver) Get(ctx context.Context, id string) (interface{}, error) {
	return c.getFromCollection(ctx, "cache", id)
}

// ListKeys returns all keys in the cache
func (c *CacheDriver) ListKeys(ctx context.Context) ([]string, error) {
	var keys []string
	var items []CacheItem

	err := c.store.View(func(tx *apitobolt.Tx) error {
		cacheCol := tx.Collection("cache")
		return cacheCol.All(&items)
	})

	if err != nil {
		return keys, err
	}

	for _, item := range items {
		keys = append(keys, item.ID)
	}

	return keys, nil
}

// AppCacheItem represents an application cache entry
type AppCacheItem struct {
	ID    string                   `json:"id"`
	Cache *models.ApplicationCache `json:"cache"`
}

// GetAppCache retrieves application cache for a project
func (c *CacheDriver) GetAppCache(ctx context.Context, projectID string) (*models.ApplicationCache, error) {
	key := c.appCacheKey(projectID)

	var item AppCacheItem
	err := c.store.View(func(tx *apitobolt.Tx) error {
		appCacheCol := tx.Collection("app_cache")

		if err := appCacheCol.FindByID(key, &item); err != nil {
			return errors.New("app cache not found")
		}

		// Check if expired
		expired, err := c.isExpired(ctx, key)
		if err != nil {
			return err
		}
		if expired {
			return errors.New("app cache expired")
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Restore the project from project cache if present
	if val, ok := c.ProjectCache.Load(projectID); ok && val != nil && item.Cache != nil {
		item.Cache.Project = val.(*models.Project)
	}

	return item.Cache, nil
}

// PutAppCache stores application cache for a project
func (c *CacheDriver) PutAppCache(ctx context.Context, projectID string, cache *models.ApplicationCache) error {
	key := c.appCacheKey(projectID)
	ttl := c.parseTTL()

	err := c.store.Update(func(tx *apitobolt.Tx) error {
		appCacheCol := tx.Collection("app_cache")

		// Store project in memory cache
		c.ProjectCache.Store(projectID, cache.Project)

		// Create a copy without the Project to avoid duplication
		_cache := *cache
		_cache.Project = nil

		item := AppCacheItem{
			ID:    key,
			Cache: &_cache,
		}

		if _, err := appCacheCol.Save(&item); err != nil {
			return fmt.Errorf("failed to store app cache: %w", err)
		}

		// Set expiration
		return c.setExpiration(ctx, key, ttl)
	})

	return err
}

// Expire removes a key from cache and its related data
func (c *CacheDriver) Expire(ctx context.Context, id string) error {
	err := c.store.Update(func(tx *apitobolt.Tx) error {
		// Remove from cache collection
		cacheCol := tx.Collection("cache")
		cacheCol.DeleteStruct(&CacheItem{ID: id})

		// Remove from app cache collection
		appCacheCol := tx.Collection("app_cache")
		appCacheKey := c.appCacheKey(id)
		appCacheCol.DeleteStruct(&AppCacheItem{ID: appCacheKey})

		// Remove from projects collection
		projectsCol := tx.Collection("projects")
		projectsCol.DeleteStruct(&ProjectItem{ID: id})

		// Remove expiration
		expCol := tx.Collection("expirations")
		expCol.DeleteStruct(&CacheExpiration{ID: id})
		expCol.DeleteStruct(&CacheExpiration{ID: appCacheKey})

		return nil
	})

	// Also remove from memory cache
	c.ProjectCache.Delete(id)

	return err
}

// ProjectItem represents a project cache entry
type ProjectItem struct {
	ID      string          `json:"id"`
	Project *models.Project `json:"project"`
}

// GetProject retrieves a project from cache
func (c *CacheDriver) GetProject(ctx context.Context, projectID string) (*models.Project, error) {
	var item ProjectItem
	err := c.store.View(func(tx *apitobolt.Tx) error {
		projectsCol := tx.Collection("projects")

		if err := projectsCol.FindByID(projectID, &item); err != nil {
			return errors.New("project not found")
		}

		// Check if expired
		expired, err := c.isExpired(ctx, projectID)
		if err != nil {
			return err
		}
		if expired {
			return errors.New("project cache expired")
		}

		return nil
	})

	return item.Project, err
}

// SaveProject stores a project in cache
func (c *CacheDriver) SaveProject(ctx context.Context, project *models.Project) (*models.Project, error) {
	ttl := c.parseTTL()

	err := c.store.Update(func(tx *apitobolt.Tx) error {
		projectsCol := tx.Collection("projects")

		item := ProjectItem{
			ID:      project.ID,
			Project: project,
		}

		if _, err := projectsCol.Save(&item); err != nil {
			return fmt.Errorf("failed to store project: %w", err)
		}

		// Set expiration
		return c.setExpiration(ctx, project.ID, ttl)
	})

	return project, err
}

// Helper methods

// putWithTTL stores a value in a specific collection with TTL
func (c *CacheDriver) putWithTTL(ctx context.Context, collectionName, key string, value interface{}, ttl time.Duration) error {
	return c.store.Update(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)

		item := CacheItem{
			ID:   key,
			Data: value,
		}

		if _, err := col.Save(&item); err != nil {
			return fmt.Errorf("failed to store value: %w", err)
		}

		// Set expiration
		return c.setExpiration(ctx, key, ttl)
	})
}

// getFromCollection retrieves a value from a specific collection
func (c *CacheDriver) getFromCollection(ctx context.Context, collectionName, key string) (interface{}, error) {
	var item CacheItem
	err := c.store.View(func(tx *apitobolt.Tx) error {
		col := tx.Collection(collectionName)

		if err := col.FindByID(key, &item); err != nil {
			return errors.New("key not found")
		}

		// Check if expired
		expired, err := c.isExpired(ctx, key)
		if err != nil {
			return err
		}
		if expired {
			return errors.New("key expired")
		}

		return nil
	})

	return item.Data, err
}

// appCacheKey generates a key for app cache
func (c *CacheDriver) appCacheKey(projectID string) string {
	return fmt.Sprintf("appcache:%s", projectID)
}

// idMaker generates a key (for compatibility with badger driver)
func (c *CacheDriver) idMaker(projectID string) string {
	return projectID
}

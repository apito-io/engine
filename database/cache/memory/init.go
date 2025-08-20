// Package memoryCache provides an in-memory cache driver with TTL support
package memoryCache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/puzpuzpuz/xsync/v3"
)

type CacheDriver struct {
	cfg   *models.Config
	Cache *xsync.Map
}

type memoryEntry struct {
	value     interface{}
	expiresAt time.Time
}

func GetMemoryCacheDriver(cfg *models.Config) (*CacheDriver, error) {
	return &CacheDriver{Cache: xsync.NewMap(), cfg: cfg}, nil
}

func (b *CacheDriver) Put(ctx context.Context, id string, cache interface{}) error {
	ttl := b.parseTTL()
	b.Cache.Store(id, memoryEntry{value: cache, expiresAt: time.Now().Add(ttl)})
	return nil
}

func (b *CacheDriver) Get(ctx context.Context, id string) (interface{}, error) {
	if val, ok := b.Cache.Load(id); ok && val != nil {
		if entry, ok := val.(memoryEntry); ok {
			if time.Now().After(entry.expiresAt) {
				b.Cache.Delete(id)
				return nil, errors.New("cache expired. fetch one")
			}
			return entry.value, nil
		}
		// backward compatibility for any previously stored raw values
		return val, nil
	}
	return nil, errors.New("cache not found. fetch one")
}

func (b *CacheDriver) ListKeys(ctx context.Context) ([]string, error) {
	var _keys []string
	b.Cache.Range(func(key string, value interface{}) bool {
		_keys = append(_keys, key)
		return true
	})
	return _keys, nil
}

func (b *CacheDriver) GetAppCache(ctx context.Context, projectID string) (*models.ApplicationCache, error) {
	key := b.appCacheKey(projectID)
	if val, ok := b.Cache.Load(key); ok && val != nil {
		if entry, ok := val.(memoryEntry); ok {
			if time.Now().After(entry.expiresAt) {
				b.Cache.Delete(key)
				return nil, errors.New("app cache expired. fetch one")
			}
			if entry.value == nil {
				return nil, nil
			}
			cache := entry.value.(*models.ApplicationCache)
			// Restore the project from project cache if present
			if p, err := b.GetProject(ctx, projectID); err == nil && p != nil {
				cache.Project = p
			}
			return cache, nil
		}
	}
	return nil, errors.New("app cache not found. fetch one")
}

func (b *CacheDriver) PutAppCache(ctx context.Context, projectID string, cache *models.ApplicationCache) error {
	key := b.appCacheKey(projectID)
	ttl := b.parseTTL()
	// store a shallow copy without the Project to avoid duplication
	_cache := *cache
	_cache.Project = nil
	b.Cache.Store(key, memoryEntry{value: &_cache, expiresAt: time.Now().Add(ttl)})
	return nil
}

func (b *CacheDriver) Expire(ctx context.Context, id string) error {
	// expire both possible keys
	b.Cache.Delete(id)
	b.Cache.Delete(b.appCacheKey(id))
	b.Cache.Delete(b.projectKey(id))
	return nil
}

func (b *CacheDriver) GetProject(ctx context.Context, projectID string) (*models.Project, error) {
	key := b.projectKey(projectID)
	if val, ok := b.Cache.Load(key); ok && val != nil {
		if entry, ok := val.(memoryEntry); ok {
			if time.Now().After(entry.expiresAt) {
				b.Cache.Delete(key)
				return nil, errors.New("project cache expired. fetch one")
			}
			if entry.value == nil {
				return nil, errors.New("project cache not found. fetch one")
			}
			return entry.value.(*models.Project), nil
		}
		// backward compatibility
		return val.(*models.Project), nil
	}
	return nil, errors.New("project cache not found. fetch one")
}

func (b *CacheDriver) SaveProject(ctx context.Context, project *models.Project) (*models.Project, error) {
	key := b.projectKey(project.ID)
	ttl := b.parseTTL()
	b.Cache.Store(key, memoryEntry{value: project, expiresAt: time.Now().Add(ttl)})
	return project, nil
}

func (b *CacheDriver) idMaker(projectID string) string {
	return projectID
}

func (b *CacheDriver) appCacheKey(projectID string) string {
	return fmt.Sprintf("appcache:%s", projectID)
}

func (b *CacheDriver) projectKey(projectID string) string {
	return fmt.Sprintf("project:%s", projectID)
}

func (b *CacheDriver) parseTTL() time.Duration {
	// default to 600 seconds if not set or invalid
	if b.cfg == nil || b.cfg.CacheTTL == "" {
		return 600 * time.Second
	}
	// First, try Go duration strings like "10m", "1h", "30s"
	if d, err := time.ParseDuration(b.cfg.CacheTTL); err == nil && d > 0 {
		return d
	}
	// Fallback: treat numeric value as seconds
	ttlSeconds, err := strconv.Atoi(b.cfg.CacheTTL)
	if err != nil || ttlSeconds <= 0 {
		return 600 * time.Second
	}
	return time.Duration(ttlSeconds) * time.Second
}

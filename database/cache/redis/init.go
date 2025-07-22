package redisCache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/apito-io/engine/models"
	"github.com/nitishm/go-rejson/v4"
	"github.com/redis/go-redis/v9"
)

type CacheDriver struct {
	cfg *models.Config
	Db  *redis.Client
	//rh           *rejson.Handler
	ProjectCache sync.Map
}

func (b *CacheDriver) Put(ctx context.Context, id string, cache interface{}) error {
	ttl, _ := strconv.Atoi(b.cfg.CacheTTL)
	data, err := json.Marshal(cache)
	if err != nil {
		fmt.Println("error marshalling cache", err.Error())
		return err
	}
	err = b.Db.Set(ctx, id, data, time.Duration(ttl)*time.Minute).Err()
	if err != nil {
		return err
	}
	return nil
}

func (b *CacheDriver) Get(ctx context.Context, id string) (interface{}, error) {
	val, err := b.Db.Get(ctx, id).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	var data interface{}
	err = json.Unmarshal([]byte(val), &data)
	if err != nil {
		return nil, err
	}
	return data, err
}

func GetRedisCacheDriver(cfg *models.Config) (*CacheDriver, error) {

	dbNo, err := strconv.Atoi(cfg.KVStorageEngineDatabase)
	if err != nil {
		return nil, err
	}

	cli := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.KVStorageEngineHost, cfg.KVStorageEnginePort),
		Password: cfg.KVStorageEnginePassword, // no password set
		DB:       dbNo,                        // use default DB
	})
	if err = cli.Ping(context.TODO()).Err(); err != nil {
		return nil, errors.New(fmt.Sprintf(`redis Cache driver error : %s`, err))
	}
	rh := rejson.NewReJSONHandler()
	rh.SetGoRedisClientWithContext(context.Background(), cli)
	//defer db.Close()

	return &CacheDriver{
		cfg: cfg,
		Db:  cli,
		//rh: rh,
		ProjectCache: sync.Map{},
	}, nil
}

func (b *CacheDriver) ListKeys(ctx context.Context) ([]string, error) {
	//TODO implement me
	panic("implement me")
}

func (b *CacheDriver) GetAppCache(ctx context.Context, projectId string) (*models.ApplicationCache, error) {
	_id := b.idMaker(projectId)

	res, err := b.Db.Get(ctx, _id).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	var cache *models.ApplicationCache
	err = json.Unmarshal([]byte(res), &cache)
	if err != nil {
		return nil, err
	}

	// restore the cache
	_val, ok := b.ProjectCache.Load(projectId)
	if ok && cache != nil && _val != nil {
		cache.Project = _val.(*models.Project)
	}

	return cache, err
}

func (b *CacheDriver) PutAppCache(ctx context.Context, projectId string, cache *models.ApplicationCache) error {
	_id := b.idMaker(projectId)

	b.ProjectCache.Store(projectId, cache.Project)
	_cache := *cache
	_cache.Project = nil

	ttl, _ := strconv.Atoi(b.cfg.CacheTTL)
	_, err := b.Db.Set(ctx, _id, _cache, time.Duration(ttl)*time.Minute).Result()
	if err != nil {
		return err
	}
	return err
}

func (b *CacheDriver) Expire(ctx context.Context, projectId string) error {
	err := b.Db.Del(ctx, projectId).Err()
	if err != nil {
		return err
	}
	// expire the local map
	return nil
}

func (b *CacheDriver) GetProject(ctx context.Context, projectId string) (*models.Project, error) {
	res, err := b.Db.Get(ctx, projectId).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, err
	}
	var cache *models.Project
	err = json.Unmarshal([]byte(res), &cache)
	if err != nil {
		return nil, err
	}

	return cache, err
}

func (b *CacheDriver) SaveProject(ctx context.Context, project *models.Project) (*models.Project, error) {
	data, err := json.Marshal(project)
	if err != nil {
		return nil, err
	}
	ttl, _ := strconv.Atoi(b.cfg.CacheTTL)
	_, err = b.Db.Set(ctx, project.ID, data, time.Duration(ttl)*time.Second).Result()
	if err != nil {
		return nil, err
	}
	return project, err
}

func (b *CacheDriver) idMaker(projectId string) string {
	return fmt.Sprintf(`%s`, projectId)
}

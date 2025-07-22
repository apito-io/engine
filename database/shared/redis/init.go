package sharedRedis

import (
	"context"
	"fmt"
	"github.com/apito-io/engine/models"
	"github.com/nitishm/go-rejson/v4"
	"github.com/pkg/errors"
	goredis "github.com/redis/go-redis/v9"
	"sync"
)

type SharedRedis struct {
	Db           *goredis.Client
	rh           *rejson.Handler
	ProjectCache sync.Map
}

func GetSharedRedisDriver(cfg *models.DriverCredentials) (*SharedRedis, error) {

	// Open the Badger database located in the /tmp/badger directory.
	// It will be created if it doesn't exist.
	cli := goredis.NewClient(&goredis.Options{
		Addr:     fmt.Sprintf("%s:%s", cfg.Host, cfg.Port),
		Password: cfg.Password, // no password set
		DB:       0,            // use default DB
	})
	if err := cli.Ping(context.TODO()).Err(); err != nil {
		return nil, err
	}
	rh := rejson.NewReJSONHandler()
	rh.SetGoRedisClientWithContext(context.Background(), cli)
	//defer db.Close()

	return &SharedRedis{Db: cli, rh: rh, ProjectCache: sync.Map{}}, nil
}

func (b *SharedRedis) Get(id string) (interface{}, error) {
	val, err := b.Db.Get(context.Background(), id).Result()
	if errors.Is(err, goredis.Nil) {
		return nil, errors.New("key not found")
	} else if err != nil {
		return nil, err
	}
	return val, err
}

func (b *SharedRedis) Set(id string, data interface{}) error {
	err := b.Db.Set(context.Background(), id, data, 0).Err()
	if err != nil {
		return err
	}
	return err
}

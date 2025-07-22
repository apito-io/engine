package badger

import (
	"context"
	"errors"
	"github.com/dgraph-io/badger/v4"
	"time"
)

func (k *KVBadgerDriver) AddToSortedSets(ctx context.Context, setName string, key string, exp time.Duration) error {
	//TODO implement me
	panic("implement me")
}

func (k *KVBadgerDriver) GetFromSortedSets(ctx context.Context, setName string, key string) (float64, error) {
	//TODO implement me
	panic("implement me")
}

func (k *KVBadgerDriver) SetToHashMap(ctx context.Context, hash, key string, value string) error {
	//TODO implement me
	panic("implement me")
}

func (k *KVBadgerDriver) GetFromHashMap(ctx context.Context, hash, key string) (string, error) {
	//TODO implement me
	panic("implement me")
}

func (k *KVBadgerDriver) CheckKeyHashMap(ctx context.Context, hash, key string) bool {
	//TODO implement me
	panic("implement me")
}

func (k *KVBadgerDriver) DelValue(ctx context.Context, key string) error {
	//TODO implement me
	panic("implement me")
}

func (k *KVBadgerDriver) SetValue(ctx context.Context, key string, value string, expiration time.Duration) error {
	return k.Db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), []byte(value)).WithTTL(expiration)
		err := txn.SetEntry(e)
		return err
	})
}

func (k *KVBadgerDriver) SetJSONObject(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	//TODO implement me
	panic("implement me")
}

func (k *KVBadgerDriver) GetJSONObject(ctx context.Context, key string) (interface{}, error) {
	//TODO implement me
	panic("implement me")
}

func (k *KVBadgerDriver) CheckRedisKey(ctx context.Context, keys ...string) (bool, error) {
	//TODO implement me
	panic("implement me")
}

func (k *KVBadgerDriver) GetValue(ctx context.Context, key string) (string, error) {
	var _val string
	err := k.Db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			if errors.Is(err, badger.ErrKeyNotFound) {
				return nil
			} else {
				return err
			}
		}

		err = item.Value(func(val []byte) error {
			_val = string(val)
			return nil
		})
		if err != nil {
			return err
		}
		return nil
	})
	return _val, err
}

func (k *KVBadgerDriver) AddToSets(ctx context.Context, key string, value string) error {
	//TODO implement me
	panic("implement me")
}

func (k *KVBadgerDriver) RemoveSets(ctx context.Context, key string, value string) error {
	//TODO implement me
	panic("implement me")
}

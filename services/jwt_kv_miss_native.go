//go:build !cloudflare

package services

import (
	"errors"

	"github.com/redis/go-redis/v9"
)

func isKVKeyMissing(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "key not found" || err.Error() == "key expired" || errors.Is(err, redis.Nil)
}

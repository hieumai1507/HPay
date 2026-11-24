// Package redisclient owns the Redis connection (streams, asynq, cache).
package redisclient

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// Connect creates a Redis client and verifies connectivity with a ping.
func Connect(ctx context.Context, addr string) (*redis.Client, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("ping redis %s: %w", addr, err)
	}
	return rdb, nil
}

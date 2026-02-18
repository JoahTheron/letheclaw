package services

import (
	"context"
	"fmt"

	"github.com/letheclaw/api/models"
	"github.com/redis/go-redis/v9"
)

// InitRedis initializes the Redis client
func InitRedis(config models.RedisConfig) (*redis.Client, error) {
	opts, err := redis.ParseURL(config.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	opts.PoolSize = config.MaxConnections

	client := redis.NewClient(opts)

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping Redis: %w", err)
	}

	return client, nil
}

package redis

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tf-vishal/zeotap-ims/internal/config"
)

// Client wraps the go-redis client and exposes a health check.
type Client struct {
	RDB *redis.Client
}

// NewClient creates and pings a new Redis connection.
func NewClient(cfg *config.Config) (*Client, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		PoolSize:     200,              // sized for 50 stream workers + overhead
		MinIdleConns: 30,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	log.Printf("[redis] connected to %s (db=%d)", cfg.RedisAddr, cfg.RedisDB)
	return &Client{RDB: rdb}, nil
}

// Ping checks if Redis is reachable (used by health endpoint).
func (c *Client) Ping(ctx context.Context) error {
	return c.RDB.Ping(ctx).Err()
}

// Close gracefully shuts down the Redis connection pool.
func (c *Client) Close() error {
	return c.RDB.Close()
}

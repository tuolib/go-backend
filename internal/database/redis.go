package database

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// NewRedis 创建 go-redis 客户端。Redis 用于缓存、购物车、库存预扣、分布式锁、限流等。
// NewRedis creates a go-redis client. Redis is used for caching, cart, stock reservation, distributed locks, rate-limiting, etc.
//
// go-redis 内部也维护连接池（默认 10 个连接），不需要手动管理。
// go-redis internally maintains a connection pool (default 10 connections) — no manual management needed.
func NewRedis(redisURL string) (*redis.Client, error) {
	// ParseURL 解析 Redis 连接字符串（如 redis://localhost:6379 或 redis://:password@host:6379/0）。
	// ParseURL parses a Redis connection string (e.g. redis://localhost:6379 or redis://:password@host:6379/0).
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}

	// 连接池参数调优 / Connection pool tuning
	opts.PoolSize = 20          // 最大连接数 / Max connections
	opts.MinIdleConns = 5       // 最小空闲连接 / Min idle connections
	opts.MaxRetries = 3         // 自动重试次数（网络抖动时有用）/ Auto-retry count (useful during network hiccups)
	opts.DialTimeout = 5 * time.Second  // 连接超时 / Connection timeout
	opts.ReadTimeout = 3 * time.Second  // 读超时 / Read timeout
	opts.WriteTimeout = 3 * time.Second // 写超时 / Write timeout

	client := redis.NewClient(opts)

	// Ping 验证 Redis 可达。
	// Ping verifies Redis is reachable.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return client, nil
}

// RedisHealthCheck 执行 Redis 健康检查。
// RedisHealthCheck performs a Redis health check.
func RedisHealthCheck(ctx context.Context, client *redis.Client) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return client.Ping(ctx).Err()
}

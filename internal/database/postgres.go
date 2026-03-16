package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool 创建 pgxpool 连接池。pgxpool 内部管理多个数据库连接，自动复用和回收。
// NewPool creates a pgxpool connection pool. pgxpool manages multiple DB connections internally, auto-reusing and recycling them.
//
// 为什么用连接池而不是单连接？每次请求都创建/关闭连接开销很大（TCP 握手 + TLS + 认证）。
// 连接池预先维护一组连接，请求时从池中借出，用完归还。
// Why a pool instead of a single connection? Creating/closing connections per request is expensive (TCP handshake + TLS + auth).
// A pool maintains a set of connections — borrow on request, return when done.
func NewPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	// ParseConfig 解析连接字符串（如 postgresql://user:pass@host:5432/dbname）为结构化配置。
	// ParseConfig parses the connection string (e.g. postgresql://user:pass@host:5432/dbname) into structured config.
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	// 连接池参数调优 / Connection pool tuning
	config.MaxConns = 20                       // 最大连接数：防止打满数据库连接 / Max connections: prevent exhausting DB connections
	config.MinConns = 5                        // 最小空闲连接：避免冷启动延迟 / Min idle connections: avoid cold-start latency
	config.MaxConnLifetime = 30 * time.Minute  // 连接最大存活时间：定期轮换防止连接老化 / Max conn lifetime: rotate to prevent stale connections
	config.MaxConnIdleTime = 5 * time.Minute   // 空闲连接超时：释放不需要的连接 / Idle timeout: release unused connections
	config.HealthCheckPeriod = 30 * time.Second // 健康检查间隔：定期 ping 剔除坏连接 / Health check interval: periodically ping to remove bad connections

	// ConnectConfig 创建连接池并立即验证至少一个连接可用。
	// ConnectConfig creates the pool and immediately verifies at least one connection works.
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("create connection pool: %w", err)
	}

	// Ping 验证数据库可达，快速失败比运行时发现连接问题好。
	// Ping verifies the database is reachable — fail fast is better than discovering connection issues at runtime.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

// HealthCheck 执行一次数据库健康检查，适合暴露给 /health 端点。
// HealthCheck performs a database health check, suitable for exposing via a /health endpoint.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	// 使用带超时的 context 防止健康检查本身卡死。
	// Use a context with timeout to prevent the health check itself from hanging.
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return pool.Ping(ctx)
}

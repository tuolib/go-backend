package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"go-backend/internal/config"
	"go-backend/internal/database"
	"go-backend/internal/gateway"
)

func main() {
	// ── 1. 加载配置 ───────────────────────────────────────────
	// ── 1. Load configuration ─────────────────────────────────
	k, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	cfg := config.LoadGateway(k)

	// ── 2. 连接 Redis ─────────────────────────────────────────
	// ── 2. Connect to Redis ───────────────────────────────────
	//
	// Gateway 用 Redis 做限流和 token 黑名单检查。
	// Gateway uses Redis for rate limiting and token blacklist checks.
	//
	// Redis 不可用时降级运行：redisClient 为 nil，中间件会跳过 Redis 相关逻辑。
	// Degrades gracefully when Redis is unavailable: redisClient is nil, middleware skips Redis logic.
	var redisClient = initRedis(cfg.Redis.URL)

	// ── 3. 组装 Gateway ───────────────────────────────────────
	// ── 3. Assemble Gateway ───────────────────────────────────
	handler, err := gateway.NewServer(cfg, redisClient)
	if err != nil {
		slog.Error("failed to create gateway server", "error", err)
		os.Exit(1)
	}

	// ── 4. 启动 HTTP 服务 ─────────────────────────────────────
	// ── 4. Start HTTP server ──────────────────────────────────
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second, // 防止 Slowloris 攻击 / Prevent Slowloris attack
	}

	// 监听系统信号，用于优雅关停。
	// Listen for OS signals for graceful shutdown.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 在独立的 goroutine 中启动 HTTP 服务。
	// Start HTTP server in a separate goroutine.
	go func() {
		slog.Info("api gateway starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done() // 阻塞等待信号 / Block until signal
	slog.Info("shutting down api gateway")

	// 优雅关停：给处理中的请求 10 秒完成。
	// Graceful shutdown: give in-flight requests 10 seconds to finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

// initRedis 尝试连接 Redis，失败时返回 nil（降级运行）。
// initRedis attempts to connect to Redis; returns nil on failure (degraded mode).
func initRedis(redisURL string) *redis.Client {
	if redisURL == "" {
		slog.Warn("redis url not configured, running without redis")
		return nil
	}
	client, err := database.NewRedis(redisURL)
	if err != nil {
		slog.Warn("failed to connect to redis, running without redis", "error", err)
		return nil
	}
	slog.Info("redis connected")
	return client
}

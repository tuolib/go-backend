package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"go-backend/internal/auth"
	"go-backend/internal/config"
	"go-backend/internal/database"
	"go-backend/internal/handler"
	"go-backend/internal/middleware"
	"go-backend/internal/response"
	userhandler "go-backend/internal/user/handler"
	"go-backend/internal/user/repository"
	"go-backend/internal/user/service"
)

func main() {
	// ── 1. 加载配置 ───────────────────────────────────────────
	// ── 1. Load configuration ─────────────────────────────────
	k, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	cfg := config.LoadUser(k)

	// ── 2. 连接数据库 ─────────────────────────────────────────
	// ── 2. Connect to database ────────────────────────────────
	//
	// 用户服务必须有数据库 — 没有 PG 就无法存储用户数据，直接退出。
	// User service requires a database — without PG there's no user data storage, so exit immediately.
	pool, err := database.NewPool(context.Background(), cfg.Postgres.URL)
	if err != nil {
		slog.Error("failed to connect to postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// ── 3. 连接 Redis ─────────────────────────────────────────
	// ── 3. Connect to Redis ───────────────────────────────────
	//
	// Redis 用于 token 黑名单和限流。不可用时降级运行。
	// Redis is used for token blacklist and rate limiting. Degrades gracefully when unavailable.
	redisClient := initRedis(cfg.Redis.URL)

	// ── 4. 构建基础设施 ───────────────────────────────────────
	// ── 4. Build infrastructure ───────────────────────────────
	jwtManager := auth.NewJWTManager(
		cfg.JWT.AccessSecret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessExpiresIn,
		cfg.JWT.RefreshExpiresIn,
	)
	hasher := auth.NewArgon2Hasher()

	// ── 5. 构建三层架构：Repository → Service → Handler ──────
	// ── 5. Build three-layer architecture: Repository → Service → Handler ──
	//
	// 依赖注入方向：Handler 依赖 Service，Service 依赖 Repository。
	// 构造顺序是反过来的：先建底层，再建上层。
	// Dependency direction: Handler → Service → Repository.
	// Construction order is reversed: build bottom layer first, then upper layers.

	// Repository 层（数据库操作）
	// Repository layer (database operations)
	userRepo := repository.NewUserRepository(pool)
	addrRepo := repository.NewAddressRepository(pool)
	tokenRepo := repository.NewTokenRepository(pool)

	// Service 层（业务逻辑）
	// Service layer (business logic)
	authService := service.NewAuthService(userRepo, tokenRepo, jwtManager, hasher, redisClient)
	userService := service.NewUserService(userRepo)
	addrService := service.NewAddressService(addrRepo)

	// Handler 层（HTTP 处理）
	// Handler layer (HTTP handling)
	authHandler := userhandler.NewAuthHandler(authService)
	userHandler := userhandler.NewUserHandler(userService)
	addrHandler := userhandler.NewAddressHandler(addrService)

	// ── 6. 组装路由 ────────────────────────────────────────────
	// ── 6. Assemble routes ─────────────────────────────────────
	r := chi.NewRouter()

	// 全局中间件（所有请求都经过）
	// Global middleware (applied to all requests)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.RateLimit(middleware.RateLimitConfig{
		RedisClient: redisClient,
		WindowSize:  60 * time.Second,
		MaxRequests: 100,
	}))

	// 健康检查（不需要认证）
	// Health check (no auth required)
	r.Post("/health", handler.Wrap(healthHandler))

	// ── 6a. 公开路由（无需认证）─────────────────────────────
	// ── 6a. Public routes (no auth required) ─────────────────
	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/register", handler.Wrap(authHandler.Register))
		r.Post("/auth/login", handler.Wrap(authHandler.Login))
		r.Post("/auth/refresh", handler.Wrap(authHandler.Refresh))

		// ── 6b. 需要认证的路由 ──────────────────────────────
		// ── 6b. Authenticated routes ─────────────────────────
		//
		// r.Group 创建一个子路由组，共享中间件但不影响外层。
		// r.Group creates a sub-router group that shares middleware without affecting the outer scope.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(middleware.AuthConfig{
				JWTManager:  jwtManager,
				RedisClient: redisClient,
			}))

			// Auth（需要认证的）
			// Auth (requires authentication)
			r.Post("/auth/logout", handler.Wrap(authHandler.Logout))

			// User
			r.Post("/user/profile", handler.Wrap(userHandler.Profile))
			r.Post("/user/update", handler.Wrap(userHandler.Update))

			// Address
			r.Post("/user/address/list", handler.Wrap(addrHandler.List))
			r.Post("/user/address/create", handler.Wrap(addrHandler.Create))
			r.Post("/user/address/update", handler.Wrap(addrHandler.Update))
			r.Post("/user/address/delete", handler.Wrap(addrHandler.Delete))
		})
	})

	// ── 6c. 内部接口（服务间调用）────────────────────────────
	// ── 6c. Internal endpoints (inter-service calls) ─────────
	//
	// InternalOnly 中间件校验 X-Internal-Secret，只允许持有密钥的服务调用。
	// Docker 网络层也会阻止外部请求到达这些端口。
	// InternalOnly middleware verifies X-Internal-Secret — only services with the secret can call these.
	// Docker networking also prevents external requests from reaching these ports.
	r.Route("/internal", func(r chi.Router) {
		r.Use(middleware.InternalOnly(cfg.Internal.Secret))

		r.Post("/user/detail", handler.Wrap(userHandler.Detail))
		r.Post("/user/batch", handler.Wrap(userHandler.Batch))
		r.Post("/user/address/detail", handler.Wrap(addrHandler.AddressDetail))
	})

	// ── 7. 启动 HTTP 服务 ─────────────────────────────────────
	// ── 7. Start HTTP server ──────────────────────────────────
	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second, // 防止 Slowloris 攻击 / Prevent Slowloris attack
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("user service starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down user service")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

// healthHandler 用户服务健康检查。
// healthHandler is the user service health check.
func healthHandler(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, r, map[string]string{
		"service": "user",
		"status":  "ok",
	})
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

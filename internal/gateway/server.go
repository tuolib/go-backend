package gateway

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"go-backend/internal/auth"
	"go-backend/internal/config"
	"go-backend/internal/handler"
	"go-backend/internal/middleware"
)

// NewServer 组装 API Gateway 的完整路由和中间件链。
// NewServer assembles the complete API Gateway router with its full middleware chain.
//
// 这是 Gateway 的「装配车间」，把独立的零件组装成一台完整的机器：
// This is the Gateway's "assembly shop" — it takes independent parts and assembles them into a complete machine:
//
//	Registry（路由表）+ Proxy（反向代理）+ Middleware（中间件链）→ http.Handler
//	Registry (route table) + Proxy (reverse proxy) + Middleware (middleware chain) → http.Handler
func NewServer(cfg config.GatewayConfig, redisClient *redis.Client) (http.Handler, error) {

	// ── 1. 构建服务注册表 ──────────────────────────────────────
	// ── 1. Build the service registry ──────────────────────────
	//
	// 把路由前缀和下游服务地址一一对应。
	// Map route prefixes to downstream service URLs.
	reg := NewRegistry()

	// 注册顺序决定匹配优先级（先注册的先匹配）。
	// Registration order determines match priority (first registered wins).
	services := []struct {
		prefix string
		url    string
	}{
		{"/api/v1/user/", cfg.UserServiceURL},
		{"/api/v1/product/", cfg.ProductServiceURL},
		{"/api/v1/cart/", cfg.CartServiceURL},
		{"/api/v1/order/", cfg.OrderServiceURL},
	}

	for _, s := range services {
		if s.url == "" {
			continue // 该服务未配置（单体模式下不需要）/ Service not configured (not needed in monolith mode)
		}
		if err := reg.Register(s.prefix, s.url); err != nil {
			return nil, err
		}
	}

	// ── 2. 构建 JWT 管理器 ─────────────────────────────────────
	// ── 2. Build JWT manager ───────────────────────────────────
	//
	// Gateway 需要 JWT 管理器来验证 access token（Auth 中间件用）。
	// Gateway needs a JWT manager to verify access tokens (used by Auth middleware).
	// Gateway 级别暂不直接使用 JWTManager — Auth 中间件由下游服务各自挂载。
	// Gateway doesn't use JWTManager directly for now — Auth middleware is applied by each downstream service.
	// 保留构建逻辑：后续如果 Gateway 需要对特定路由做认证（如管理后台），在此处使用。
	// Keep the construction: if Gateway needs auth for specific routes later (e.g. admin panel), use it here.
	_ = auth.NewJWTManager(
		cfg.JWT.AccessSecret,
		cfg.JWT.RefreshSecret,
		cfg.JWT.AccessExpiresIn,
		cfg.JWT.RefreshExpiresIn,
	)

	// ── 3. 组装路由 ────────────────────────────────────────────
	// ── 3. Assemble routes ─────────────────────────────────────
	r := chi.NewRouter()

	// ── 3a. 全局中间件链 ───────────────────────────────────────
	// ── 3a. Global middleware chain ────────────────────────────
	//
	// 顺序很重要！参考 docs/phase4-overview.md 第 2 节。
	// Order matters! See docs/phase4-overview.md section 2.
	//
	//   RequestID → Logger → CORS → RateLimit → Proxy
	//
	// Auth 和 Idempotent 不在全局链中 — 它们由各下游服务按需使用。
	// Auth and Idempotent are NOT in the global chain — downstream services apply them per-route as needed.
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(middleware.CORSConfig{
		AllowedOrigins: cfg.CORS.Origins,
	}))
	r.Use(middleware.RateLimit(middleware.RateLimitConfig{
		RedisClient: redisClient,
		WindowSize:  60 * time.Second,
		MaxRequests: 100,
	}))

	// ── 3b. 健康检查（不经过代理）─────────────────────────────
	// ── 3b. Health check (bypasses proxy) ──────────────────────
	r.Post("/health", handler.Wrap(HealthHandler))

	// ── 3c. 反向代理（兜底路由）────────────────────────────────
	// ── 3c. Reverse proxy (catch-all route) ────────────────────
	//
	// chi 的 HandleFunc("/*", ...) 匹配所有未被上面路由命中的请求。
	// chi's HandleFunc("/*", ...) catches all requests not matched above.
	//
	// 请求流向：客户端 → Gateway middleware → Proxy → 下游服务
	// Request flow: Client → Gateway middleware → Proxy → downstream service
	r.HandleFunc("/*", NewProxyHandler(reg, cfg.Internal.Secret))

	return r, nil
}

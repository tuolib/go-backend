package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/redis/go-redis/v9"

	"go-backend/internal/apperr"
	"go-backend/internal/auth"
	"go-backend/internal/response"
)

// AuthConfig Auth 中间件的依赖配置。
// AuthConfig holds dependencies for the Auth middleware.
//
// 为什么用 config struct 而不是多个参数？参数超过 2-3 个时，struct 更清晰，
// 而且后续加字段不会破坏调用方的代码。
// Why a config struct instead of multiple params? With 2-3+ params, a struct is clearer,
// and adding fields later won't break callers.
type AuthConfig struct {
	JWTManager  *auth.JWTManager // JWT 签名验证器 / JWT signature verifier
	RedisClient *redis.Client    // Redis 客户端，用于检查 token 黑名单 / Redis client for token blacklist checks
}

// Auth 验证 JWT access token，将用户信息注入 context。
// Auth verifies the JWT access token and injects user info into context.
//
// 处理流程 / Flow:
//   1. 从 Authorization 头提取 Bearer token
//   2. 验证 token 签名和有效期
//   3. 查 Redis 黑名单（登出时写入）
//   4. 将 userId / email / jti 写入 context
func Auth(cfg AuthConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// --- Step 1: 从请求头提取 token ---
			// --- Step 1: Extract token from request header ---
			//
			// 格式：Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
			// Format: Authorization: Bearer eyJhbGciOiJIUzI1NiIs...
			header := r.Header.Get("Authorization")

			// strings.CutPrefix 比 TrimPrefix 更好：它还返回一个 bool 告诉你是否真的有前缀。
			// strings.CutPrefix is better than TrimPrefix: it also returns a bool indicating whether the prefix existed.
			//
			// TrimPrefix("NoBearer", "Bearer ") 会静默返回原字符串，容易产生 bug。
			// TrimPrefix("NoBearer", "Bearer ") silently returns the original string, which can cause bugs.
			tokenStr, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || tokenStr == "" {
				response.HandleError(w, r, apperr.NewUnauthorized("missing or invalid authorization header"))
				return
			}

			// --- Step 2: 验证 token 签名和有效期 ---
			// --- Step 2: Verify token signature and expiration ---
			claims, err := cfg.JWTManager.VerifyAccessToken(tokenStr)
			if err != nil {
				response.HandleError(w, r, apperr.NewUnauthorized("invalid or expired token"))
				return
			}

			// --- Step 3: 检查 Redis 黑名单 ---
			// --- Step 3: Check Redis blacklist ---
			//
			// 登出时把 token 的 JTI 存入 Redis（key: blacklist:jti:<jti>）。
			// On logout, the token's JTI is stored in Redis (key: blacklist:jti:<jti>).
			//
			// 为什么用 JTI 而不是整个 token？JTI 是 21 字符的 nanoid，token 可能有几百字节，
			// 用 JTI 省空间、查询快。
			// Why JTI instead of the full token? JTI is a 21-char nanoid; tokens can be hundreds of bytes.
			// JTI saves space and is faster to look up.
			if cfg.RedisClient != nil {
				blacklistKey := "blacklist:jti:" + claims.ID
				exists, err := cfg.RedisClient.Exists(r.Context(), blacklistKey).Result()
				if err != nil {
					// Redis 故障时放行，不因缓存问题阻断所有请求。
					// On Redis failure, allow through — don't block all requests due to a cache issue.
					//
					// 这是降级策略：可用性 > 安全性。最坏情况是已登出的 token 在 Redis 恢复前仍可用，
					// 但 token 本身会自然过期（15分钟）。
					// This is a degradation strategy: availability > security. Worst case: a logged-out token
					// remains usable until Redis recovers, but the token will naturally expire (15 min).
					slog.WarnContext(r.Context(), "redis blacklist check failed, allowing request", "error", err)
				} else if exists > 0 {
					response.HandleError(w, r, apperr.New(apperr.ErrCodeTokenRevoked, http.StatusUnauthorized, "token has been revoked"))
					return
				}
			}

			// --- Step 4: 将用户信息注入 context ---
			// --- Step 4: Inject user info into context ---
			//
			// 下游 handler 通过 middleware.UserIDFrom(ctx) 等 getter 读取。
			// Downstream handlers read via middleware.UserIDFrom(ctx) and similar getters.
			ctx := r.Context()
			ctx = context.WithValue(ctx, UserIDKey, claims.Subject)   // Subject = userId
			ctx = context.WithValue(ctx, UserEmailKey, claims.Email)  // 自定义字段 / Custom field
			ctx = context.WithValue(ctx, TokenJTIKey, claims.ID)      // ID = jti

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

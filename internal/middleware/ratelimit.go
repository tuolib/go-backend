package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"go-backend/internal/apperr"
	"go-backend/internal/id"
	"go-backend/internal/response"
)

// RateLimitConfig 限流中间件的配置。
// RateLimitConfig holds configuration for the rate-limit middleware.
type RateLimitConfig struct {
	RedisClient *redis.Client // Redis 客户端 / Redis client
	WindowSize  time.Duration // 滑动窗口大小（如 60s）/ Sliding window size (e.g. 60s)
	MaxRequests int           // 窗口内最大请求数 / Max requests within the window
}

// getClientIP 提取客户端真实 IP。反向代理（Caddy）会设置 X-Forwarded-For。
// getClientIP extracts the real client IP. Reverse proxies (Caddy) set X-Forwarded-For.
func getClientIP(r *http.Request) string {
	// X-Forwarded-For 格式："客户端IP, 代理1IP, 代理2IP"，取第一个。
	// X-Forwarded-For format: "clientIP, proxy1IP, proxy2IP" — take the first one.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		for i, ch := range xff {
			if ch == ',' {
				return xff[:i]
			}
		}
		return xff
	}

	if xri := r.Header.Get("X-Real-Ip"); xri != "" {
		return xri
	}

	return r.RemoteAddr
}

// RateLimit 使用 Redis ZSET 滑动窗口限流。
// RateLimit implements sliding-window rate limiting using Redis ZSET.
//
// 算法 / Algorithm:
//   1. ZREMRANGEBYSCORE — 删掉窗口外的旧记录
//   2. ZADD             — 把当前请求加入窗口
//   3. ZCARD            — 统计窗口内总请求数
//   4. PEXPIRE          — 设置 key 过期时间，防止数据永驻
//
// 用 Pipeline 把 4 条命令合并为一次网络往返，减少延迟。
// Pipeline batches 4 commands into one round trip, reducing latency.
func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// 健康检查跳过限流，避免启动探针被拦截。
			// Skip rate limiting for health checks to avoid blocking startup probes.
			if r.URL.Path == "/health" || r.URL.Path == "/health/live" {
				next.ServeHTTP(w, r)
				return
			}

			// Redis 不可用时降级放行。
			// Degrade gracefully when Redis is unavailable.
			if cfg.RedisClient == nil {
				next.ServeHTTP(w, r)
				return
			}

			ip := getClientIP(r)
			key := fmt.Sprintf("gateway:ratelimit:ip:%s", ip)
			now := time.Now()
			windowStart := now.Add(-cfg.WindowSize)

			// 用 nanoid 做 ZSET member，保证每次请求唯一。
			// Use nanoid as ZSET member to ensure uniqueness per request.
			//
			// 为什么不用时间戳做 member？同一毫秒可能有多个请求，时间戳会去重。
			// Why not use timestamp as member? Multiple requests in the same ms would deduplicate.
			member, _ := id.GenerateID()

			// Pipeline：4 条命令打包成一次 RTT。
			// Pipeline: batch 4 commands into one round trip.
			pipe := cfg.RedisClient.Pipeline()
			pipe.ZRemRangeByScore(r.Context(), key, "0", strconv.FormatFloat(float64(windowStart.UnixMilli()), 'f', 0, 64))
			pipe.ZAdd(r.Context(), key, redis.Z{Score: float64(now.UnixMilli()), Member: member})
			countCmd := pipe.ZCard(r.Context(), key)
			pipe.PExpire(r.Context(), key, cfg.WindowSize)

			_, err := pipe.Exec(r.Context())
			if err != nil {
				// Redis 故障时放行，记日志告警。
				// On Redis failure, allow through and log a warning.
				slog.WarnContext(r.Context(), "ratelimit redis pipeline failed, allowing request", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			count := countCmd.Val()
			remaining := cfg.MaxRequests - int(count)
			if remaining < 0 {
				remaining = 0
			}

			// 设置限流响应头，让客户端知道剩余额度。
			// Set rate-limit response headers so the client knows its remaining quota.
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(cfg.MaxRequests))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if int(count) > cfg.MaxRequests {
				retryAfter := int(cfg.WindowSize.Seconds())
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				response.HandleError(w, r, apperr.NewRateLimited())
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

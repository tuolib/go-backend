package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"go-backend/internal/apperr"
	"go-backend/internal/response"
)

// IdempotentConfig 幂等中间件的配置。
// IdempotentConfig holds configuration for the idempotent middleware.
type IdempotentConfig struct {
	RedisClient *redis.Client // Redis 客户端 / Redis client
	TTL         time.Duration // key 过期时间，如 24h / Key expiration, e.g. 24h
}

// Idempotent 通过 X-Idempotency-Key 请求头防止重复提交。
// Idempotent prevents duplicate submissions via the X-Idempotency-Key request header.
//
// 原理 / How it works:
//   前端对每次提交生成一个唯一 key，放在 X-Idempotency-Key 头里。
//   The frontend generates a unique key for each submission in the X-Idempotency-Key header.
//
//   后端用 Redis SET NX（不存在才设置）检查：
//   The backend uses Redis SET NX (set if not exists) to check:
//     - key 不存在 → SET 成功 → 第一次提交，放行
//       key doesn't exist → SET succeeds → first submission, allow through
//     - key 已存在 → SET 失败 → 重复提交，拒绝
//       key exists → SET fails → duplicate submission, reject
//
// 只对携带 X-Idempotency-Key 的请求生效，普通请求直接放行。
// Only applies to requests carrying X-Idempotency-Key — normal requests pass through.
func Idempotent(cfg IdempotentConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// 没有幂等 key 的请求直接放行（不是所有接口都需要幂等）。
			// Requests without an idempotency key pass through (not all endpoints need idempotency).
			idempotencyKey := r.Header.Get("X-Idempotency-Key")
			if idempotencyKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			// Redis 不可用时降级放行。
			// Degrade gracefully when Redis is unavailable.
			if cfg.RedisClient == nil {
				next.ServeHTTP(w, r)
				return
			}

			redisKey := "order:idempotent:" + idempotencyKey

			// SET NX：原子操作，key 不存在时设置并返回 true，已存在返回 false。
			// SET NX: atomic operation — sets the key and returns true if it doesn't exist, false if it does.
			//
			// 为什么用 SET NX 而不是先 GET 再 SET？
			// 因为 GET + SET 不是原子的，两个请求可能同时 GET 到"不存在"，然后都 SET 成功。
			// Why SET NX instead of GET then SET?
			// GET + SET is not atomic — two requests could both GET "not exists" and then both SET successfully.
			ok, err := cfg.RedisClient.SetNX(r.Context(), redisKey, "1", cfg.TTL).Result()
			if err != nil {
				// Redis 故障时放行，记日志告警。
				// On Redis failure, allow through and log a warning.
				slog.WarnContext(r.Context(), "idempotent redis check failed, allowing request", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			// ok=false 表示 key 已存在 → 重复提交。
			// ok=false means key already exists → duplicate submission.
			if !ok {
				response.HandleError(w, r, apperr.New(
					apperr.ErrCodeIdempotentConflict,
					http.StatusConflict,
					"duplicate submission, please do not resubmit",
				))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

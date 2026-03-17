package middleware

import "net/http"

// CORSConfig CORS 中间件的配置。
// CORSConfig holds configuration for the CORS middleware.
type CORSConfig struct {
	AllowedOrigins []string // 允许的源列表，如 ["http://localhost:5173"] / Allowed origin list
}

// CORS 处理浏览器跨域请求，设置必要的响应头。
// CORS handles browser cross-origin requests by setting the required response headers.
//
// 两种情况：
// Two scenarios:
//   1. 预检请求（OPTIONS）：浏览器先问"我能不能发请求？"，后端回答后直接 204 返回。
//      Preflight (OPTIONS): browser asks "can I send this?", backend answers and returns 204.
//   2. 正常请求：带上 CORS 头，继续走下游。
//      Normal request: attach CORS headers and pass to downstream.
func CORS(cfg CORSConfig) func(http.Handler) http.Handler {
	// 把 slice 转成 map，查找从 O(n) 变成 O(1)。
	// Convert slice to map — lookup goes from O(n) to O(1).
	allowed := make(map[string]bool, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowed[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// 只有 origin 在白名单里才设置 CORS 头。
			// Only set CORS headers if the origin is whitelisted.
			//
			// 为什么不用 "*"？因为带 credentials（cookie/token）的请求，
			// 浏览器不接受 "*"，必须回显具体的 origin。
			// Why not use "*"? Requests with credentials (cookies/tokens)
			// require the exact origin — browsers reject "*" in that case.
			if allowed[origin] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")

				// Vary: Origin 告诉缓存（CDN/浏览器）：响应因 Origin 头而异，不能混用缓存。
				// Vary: Origin tells caches (CDN/browser): response varies by Origin header — don't share cached responses.
				w.Header().Set("Vary", "Origin")
			}

			// 预检请求：浏览器发 OPTIONS 方法，带 Access-Control-Request-Method 头。
			// Preflight: browser sends OPTIONS with Access-Control-Request-Method header.
			if r.Method == http.MethodOptions {
				// 告诉浏览器允许哪些方法和头。
				// Tell the browser which methods and headers are allowed.
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-Id, X-Idempotency-Key")

				// Max-Age：预检结果缓存 12 小时，减少 OPTIONS 请求次数。
				// Max-Age: cache preflight result for 12 hours to reduce OPTIONS request frequency.
				w.Header().Set("Access-Control-Max-Age", "43200")

				// 204 No Content：预检请求不需要响应体。
				// 204 No Content: preflight requests don't need a response body.
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

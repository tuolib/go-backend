package gateway

import (
	"net/http"
	"net/http/httputil"

	"go-backend/internal/apperr"
	"go-backend/internal/middleware"
	"go-backend/internal/response"
)

// NewProxyHandler 创建反向代理 handler，根据请求路径查找注册表并转发。
// NewProxyHandler creates a reverse-proxy handler that looks up the registry by request path and forwards.
//
// 反向代理的工作方式：
// How reverse proxy works:
//   客户端 → Gateway(proxy) → 下游服务 → Gateway(proxy) → 客户端
//   Client → Gateway(proxy) → downstream → Gateway(proxy) → client
//
// 客户端不知道下游服务的存在，以为自己在和 Gateway 通信。
// The client doesn't know downstream services exist — it thinks it's talking to the Gateway.
func NewProxyHandler(reg *Registry, internalSecret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		// 在注册表中查找匹配的下游服务。
		// Look up the matching downstream service in the registry.
		entry := reg.Lookup(r.URL.Path)
		if entry == nil {
			response.HandleError(w, r, apperr.NewNotFound("route", r.URL.Path))
			return
		}

		// httputil.ReverseProxy 是 Go 标准库提供的反向代理。
		// httputil.ReverseProxy is a reverse proxy provided by Go's standard library.
		//
		// Director 函数在转发前修改请求：改目标地址、注入追踪头。
		// The Director function modifies the request before forwarding: changes the target and injects tracing headers.
		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				// 把请求的目标从 Gateway 改为下游服务。
				// Redirect the request from Gateway to the downstream service.
				req.URL.Scheme = entry.TargetURL.Scheme // http 或 https / http or https
				req.URL.Host = entry.TargetURL.Host     // 如 "user:3001" / e.g. "user:3001"
				req.Host = entry.TargetURL.Host          // Host header 也要改，否则下游收到的是 Gateway 的 host / Must also set Host header, otherwise downstream sees Gateway's host

				// 注入追踪头，让下游服务能关联请求。
				// Inject tracing headers so downstream services can correlate requests.
				ctx := req.Context()

				if traceID := middleware.TraceIDFrom(ctx); traceID != "" {
					req.Header.Set("X-Trace-Id", traceID)
				}

				// 把当前登录用户 ID 传给下游，下游不需要重新解析 JWT。
				// Pass the authenticated user ID downstream — downstream doesn't need to re-parse the JWT.
				if userID := middleware.UserIDFrom(ctx); userID != "" {
					req.Header.Set("X-User-Id", userID)
				}

				// 注入内部鉴权密钥，下游验证请求来自可信的 Gateway。
				// Inject internal auth secret — downstream verifies the request came from a trusted Gateway.
				if internalSecret != "" {
					req.Header.Set("X-Internal-Secret", internalSecret)
				}
			},
		}

		proxy.ServeHTTP(w, r)
	}
}

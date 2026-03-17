package middleware

import (
	"context"
	"net/http"

	"go-backend/internal/id"
	"go-backend/internal/response"
)

// contextKey 中间件专用的 context key 类型，和 response.ContextKey 独立。
// contextKey is the middleware-specific context key type, independent from response.ContextKey.
//
// UserID、Email、JTI 只在 middleware 写入、handler 通过 getter 读取，不需要跨包共享类型。
// UserID, Email, JTI are only written by middleware and read by handlers via getters — no need to share the type across packages.
type contextKey string

const (
	// UserIDKey 当前登录用户 ID 的 context key。auth 中间件写入，handler 读取。
	// UserIDKey is the context key for the authenticated user ID. Written by auth middleware, read by handlers.
	UserIDKey contextKey = "userId"

	// UserEmailKey 当前登录用户邮箱的 context key。auth 中间件写入。
	// UserEmailKey is the context key for the authenticated user email. Written by auth middleware.
	UserEmailKey contextKey = "userEmail"

	// TokenJTIKey 当前 token 的 JTI。auth 中间件写入，登出时用于加入黑名单。
	// TokenJTIKey is the context key for the current token's JTI. Written by auth middleware, used for blacklisting on logout.
	TokenJTIKey contextKey = "tokenJti"
)

// RequestID 生成 nanoid 作为 traceId，注入 context 并设置响应头。
// RequestID generates a nanoid as traceId, injects it into context, and sets the response header.
//
// 中间件链中必须排第一：后续所有中间件和 handler 都依赖 traceId 做日志关联。
// Must be first in the middleware chain: all subsequent middleware and handlers depend on traceId for log correlation.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 优先使用客户端传入的 X-Request-Id（方便端到端追踪），没有则生成新的。
		// Prefer the client-provided X-Request-Id (for end-to-end tracing); generate a new one if absent.
		traceID := r.Header.Get("X-Request-Id")
		if traceID == "" {
			traceID, _ = id.GenerateID()
		}

		// 使用 response.TraceIDKey 注入 context，确保 response 包的 traceIDFrom 能读到。
		// Use response.TraceIDKey to inject into context, ensuring response package's traceIDFrom can read it.
		ctx := context.WithValue(r.Context(), response.TraceIDKey, traceID)

		// 设置响应头，让前端和网络工具能看到追踪 ID。
		// Set response header so the frontend and network tools can see the trace ID.
		w.Header().Set("X-Trace-Id", traceID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TraceIDFrom 从 context 中提取 traceId。logger 中间件调用。
// TraceIDFrom extracts the traceId from context. Called by the logger middleware.
func TraceIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(response.TraceIDKey).(string); ok {
		return v
	}
	return ""
}

// UserIDFrom 从 context 中提取已认证的用户 ID。handler 层调用。
// UserIDFrom extracts the authenticated user ID from context. Called by handlers.
func UserIDFrom(ctx context.Context) string {
	if v, ok := ctx.Value(UserIDKey).(string); ok {
		return v
	}
	return ""
}

// UserEmailFrom 从 context 中提取已认证的用户邮箱。
// UserEmailFrom extracts the authenticated user email from context.
func UserEmailFrom(ctx context.Context) string {
	if v, ok := ctx.Value(UserEmailKey).(string); ok {
		return v
	}
	return ""
}

// TokenJTIFrom 从 context 中提取当前 token 的 JTI。
// TokenJTIFrom extracts the current token's JTI from context.
func TokenJTIFrom(ctx context.Context) string {
	if v, ok := ctx.Value(TokenJTIKey).(string); ok {
		return v
	}
	return ""
}

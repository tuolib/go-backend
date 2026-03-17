package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

// responseRecorder 包装 http.ResponseWriter，捕获状态码供日志使用。
// responseRecorder wraps http.ResponseWriter, capturing the status code for logging.
//
// 为什么需要它？Go 的 ResponseWriter 接口没有 getter，写入状态码后无法读取。
// Why is this needed? Go's ResponseWriter interface has no getter — once a status code is written, there's no way to read it back.
type responseRecorder struct {
	http.ResponseWriter // 嵌入原始 writer，自动继承 Header()、Write() 方法 / Embed original writer, inheriting Header() and Write()
	statusCode int      // 记录 WriteHeader 传入的状态码 / Records the status code passed to WriteHeader
}

// WriteHeader 覆盖嵌入的 WriteHeader，先记录状态码再调用原始方法。
// WriteHeader overrides the embedded WriteHeader — records the status code, then calls the original.
func (rec *responseRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

// Logger 记录每个请求的方法、路径、状态码和耗时。
// Logger logs each request's method, path, status code, and duration.
//
// 必须排在 RequestID 之后：依赖 context 中的 traceId 做日志关联。
// Must come after RequestID in the chain: depends on traceId in context for log correlation.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 用 responseRecorder 包装原始 writer，默认状态码 200。
		// Wrap the original writer with responseRecorder, defaulting to 200.
		//
		// 为什么默认 200？如果 handler 直接 Write() 不调用 WriteHeader()，
		// Go 会隐式发送 200，但我们的 recorder 不会被触发，所以需要预设。
		// Why default to 200? If a handler calls Write() without WriteHeader(),
		// Go implicitly sends 200, but our recorder wouldn't be triggered, so we preset it.
		rec := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// 把 rec 传给下游，下游调用 WriteHeader 时会触发我们覆盖的版本。
		// Pass rec downstream — when downstream calls WriteHeader, our overridden version fires.
		next.ServeHTTP(rec, r)

		// 请求处理完毕，记录日志。
		// Request complete — log the details.
		duration := time.Since(start)

		// 根据状态码选择日志级别：4xx 用 Warn，5xx 用 Error，其他用 Info。
		// Choose log level by status code: 4xx → Warn, 5xx → Error, otherwise → Info.
		level := slog.LevelInfo
		if rec.statusCode >= 500 {
			level = slog.LevelError
		} else if rec.statusCode >= 400 {
			level = slog.LevelWarn
		}

		// slog.LogAttrs 比 slog.Info/Warn/Error 更高效：避免装箱开销。
		// slog.LogAttrs is more efficient than slog.Info/Warn/Error — avoids boxing overhead.
		//
		// 用 r.Context() 传入 context，slog 会自动提取 context 中的信息。
		// Pass r.Context() so slog can extract context-bound information automatically.
		slog.LogAttrs(
			r.Context(),
			level,
			"http request",
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.statusCode),
			slog.Duration("duration", duration),
			slog.String("traceId", TraceIDFrom(r.Context())),
		)
	})
}

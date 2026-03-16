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

	"go-backend/internal/handler"
	"go-backend/internal/response"
)

func main() {
	port := envOr("USER_SERVICE_PORT", "3001")

	r := chi.NewRouter()
	r.Post("/health", handler.Wrap(healthHandler))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second, // 防止 Slowloris 攻击：有人故意慢速发送请求头来占用连接 / Prevent Slowloris: attacker slowly sends headers to hold connections open
	}

	// 监听 SIGINT(Ctrl+C) 和 SIGTERM(docker stop)，收到信号后 ctx 自动取消。
	// Listen for SIGINT (Ctrl+C) and SIGTERM (docker stop); ctx is auto-cancelled on signal.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// ListenAndServe 是阻塞调用，必须放在 goroutine 里，否则后面的信号监听代码永远执行不到。
	// ListenAndServe is blocking — must run in a goroutine, otherwise the signal handling code below never runs.
	go func() {
		slog.Info("user service starting", "port", port)
		// ErrServerClosed 是调用 Shutdown() 后的正常返回值，不是真的错误。
		// ErrServerClosed is the normal return after Shutdown() — not a real error.
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done() // 阻塞等待信号，收到后继续往下执行优雅关停 / Block until signal, then proceed to graceful shutdown
	slog.Info("shutting down user service")
	// 优雅关停：停止接受新连接，等待已有请求处理完，最多等 10 秒。
	// Graceful shutdown: stop accepting new connections, wait for in-flight requests, up to 10 seconds.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

// healthHandler 用户服务健康检查。
// healthHandler is the user service health check.
func healthHandler(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, r, map[string]string{
		"service": "user",
		"status":  "ok",
	})
}

// envOr 读取环境变量，不存在则返回默认值。
// envOr reads an env var; returns fallback if unset.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

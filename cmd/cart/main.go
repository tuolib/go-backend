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
	port := envOr("CART_SERVICE_PORT", "3003")

	r := chi.NewRouter()
	r.Post("/health", handler.Wrap(healthHandler))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second, // 防止 Slowloris 攻击 / Prevent Slowloris attack
	}

	// 监听系统信号，用于优雅关停 / Listen for OS signals for graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 在独立 goroutine 中启动服务 / Start server in a separate goroutine
	go func() {
		slog.Info("cart service starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done() // 阻塞等待信号 / Block until signal
	slog.Info("shutting down cart service")
	// 优雅关停 / Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

// healthHandler 购物车服务健康检查。
// healthHandler is the cart service health check.
func healthHandler(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, r, map[string]string{
		"service": "cart",
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

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
	port := envOr("API_GATEWAY_PORT", "3000")

	r := chi.NewRouter()

	// Global health check
	r.Post("/health", handler.Wrap(healthHandler))

	// 单体模式：所有服务的路由挂在同一个进程里，内部调用零网络开销。
	// 微服务模式下这些路由分散在各自的 cmd/*/main.go 中。
	r.Route("/api/v1/user", func(r chi.Router) {
		r.Post("/health", handler.Wrap(serviceHealth("user")))
	})
	r.Route("/api/v1/product", func(r chi.Router) {
		r.Post("/health", handler.Wrap(serviceHealth("product")))
	})
	r.Route("/api/v1/cart", func(r chi.Router) {
		r.Post("/health", handler.Wrap(serviceHealth("cart")))
	})
	r.Route("/api/v1/order", func(r chi.Router) {
		r.Post("/health", handler.Wrap(serviceHealth("order")))
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("monolith starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down monolith")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

func healthHandler(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, r, map[string]string{
		"service": "monolith",
		"status":  "ok",
	})
}

// serviceHealth 是高阶函数：返回一个闭包，闭包"记住"了 name 参数。
// 这样不用为每个服务写一个几乎一样的 handler 函数。
func serviceHealth(name string) handler.AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		return response.Success(w, r, map[string]string{
			"service": name,
			"status":  "ok",
		})
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

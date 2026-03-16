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

	// chi.NewRouter() 创建一个路由多路复用器（mux），用于将不同的 URL 路径映射到不同的 handler。
	// chi.NewRouter() creates a router multiplexer (mux) that maps different URL paths to different handlers.
	r := chi.NewRouter()

	// 全局健康检查端点 / Global health check endpoint
	r.Post("/health", handler.Wrap(healthHandler))

	// 单体模式：所有服务的路由挂在同一个进程里，内部调用零网络开销。
	// Monolith mode: all service routes live in a single process — internal calls have zero network overhead.
	//
	// 微服务模式下这些路由分散在各自的 cmd/*/main.go 中。
	// In microservice mode, these routes are spread across their own cmd/*/main.go files.
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
		ReadHeaderTimeout: 10 * time.Second, // 防止 Slowloris 攻击 / Prevent Slowloris attack
	}

	// signal.NotifyContext：监听 SIGINT(Ctrl+C) 和 SIGTERM(docker stop)，收到信号后 ctx 自动取消。
	// signal.NotifyContext: listen for SIGINT (Ctrl+C) and SIGTERM (docker stop); ctx is cancelled on signal.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop() // 释放信号监听资源 / Release signal listener resources

	// ListenAndServe 是阻塞调用，必须放在 goroutine 里，否则后面的信号监听代码永远执行不到。
	// ListenAndServe is a blocking call — it must run in a goroutine, otherwise the signal handling code below never executes.
	go func() {
		slog.Info("monolith starting", "port", port)
		// ErrServerClosed 是调用 Shutdown() 后的正常返回值，不是真的错误。
		// ErrServerClosed is the normal return after calling Shutdown() — it's not a real error.
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done() // 阻塞等待信号，收到后继续往下执行优雅关停 / Block until signal received, then proceed to graceful shutdown
	slog.Info("shutting down monolith")
	// 优雅关停：停止接受新连接，等待已有请求处理完，最多等 10 秒。
	// Graceful shutdown: stop accepting new connections, wait for in-flight requests to finish, up to 10 seconds.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	srv.Shutdown(shutdownCtx)
}

// healthHandler 全局健康检查 handler。
// healthHandler is the global health check handler.
func healthHandler(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, r, map[string]string{
		"service": "monolith",
		"status":  "ok",
	})
}

// serviceHealth 是高阶函数：返回一个闭包，闭包"记住"了 name 参数。
// serviceHealth is a higher-order function: it returns a closure that "remembers" the name parameter.
//
// 这样不用为每个服务写一个几乎一样的 handler 函数。
// This way we don't need to write a nearly identical handler function for each service.
func serviceHealth(name string) handler.AppHandler {
	return func(w http.ResponseWriter, r *http.Request) error {
		return response.Success(w, r, map[string]string{
			"service": name,
			"status":  "ok",
		})
	}
}

// envOr 读取环境变量，如果不存在或为空则返回 fallback 默认值。
// envOr reads an environment variable; returns the fallback default if unset or empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package gateway

import (
	"net/http"

	"go-backend/internal/response"
)

// HealthHandler 网关健康检查端点。
// HealthHandler is the gateway health-check endpoint.
//
// 目前只返回静态状态，后续可扩展为检查下游服务和依赖的连通性。
// Currently returns a static status; can be extended to check downstream services and dependencies.
func HealthHandler(w http.ResponseWriter, r *http.Request) error {
	return response.Success(w, r, map[string]string{
		"service": "gateway",
		"status":  "ok",
	})
}

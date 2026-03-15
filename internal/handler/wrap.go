package handler

import (
	"net/http"

	"go-backend/internal/response"
)

// AppHandler 在标准 http.HandlerFunc 基础上增加了 error 返回值。
// 为什么不用标准签名？因为标准签名 func(w, r) 没有返回值，
// 每个 handler 都要自己写 JSON 错误响应，导致大量重复代码。
// 加了 error 后，handler 只需 return err，错误处理统一在 Wrap 中完成。
type AppHandler func(w http.ResponseWriter, r *http.Request) error

// Wrap 把 AppHandler 转为标准 http.HandlerFunc，让 chi 路由器能识别。
// 核心作用：拦截 handler 返回的 error，统一转成 JSON 错误响应。
// 这样所有 handler 都不需要关心错误的序列化格式。
func Wrap(h AppHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			response.HandleError(w, r, err)
		}
	}
}

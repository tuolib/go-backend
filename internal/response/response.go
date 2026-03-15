package response

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"go-backend/internal/apperr"
)

// SuccessResp 和 ErrorResp 使用统一的信封格式，与 TypeScript 版 API 契约完全一致。
// 为什么要统一格式？前端只需一个拦截器处理所有响应：success=true 走正常逻辑，false 走错误逻辑。
type SuccessResp struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	Message string `json:"message"`
	TraceID string `json:"traceId"`
}

// ErrorMeta 提供开发者级别的错误详情，和 ErrorResp.Message（用户级）分离。
// 为什么分两层？用户看到"找不到商品"就够了，开发者还需要看到 "Not Found" 状态文本来排查问题。
type ErrorMeta struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ErrorResp is the standard error response envelope.
type ErrorResp struct {
	Code    int        `json:"code"`
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Data    any        `json:"data"`
	Meta    *ErrorMeta `json:"meta,omitempty"` // 指针+omitempty：nil 时 JSON 中不输出此字段，保持响应干净
	TraceID string     `json:"traceId"`
}

// Pagination holds paging metadata.
type Pagination struct {
	Page       int `json:"page"`
	PageSize   int `json:"pageSize"`
	Total      int `json:"total"`
	TotalPages int `json:"totalPages"`
}

// PaginatedData wraps a list of items with pagination info.
type PaginatedData struct {
	Items      any        `json:"items"`
	Pagination Pagination `json:"pagination"`
}

// Success writes a JSON success response.
func Success(w http.ResponseWriter, r *http.Request, data any) error {
	resp := SuccessResp{
		Code:    0,
		Success: true,
		Data:    data,
		Message: "ok",
		TraceID: traceIDFrom(r),
	}
	return writeJSON(w, http.StatusOK, resp)
}

// Paginated writes a paginated JSON success response.
func Paginated(w http.ResponseWriter, r *http.Request, items any, total, page, pageSize int) error {
	totalPages := 0
	if pageSize > 0 {
		totalPages = (total + pageSize - 1) / pageSize // 整数向上取整：等价于 math.Ceil(total/pageSize)，但无需浮点运算
	}

	data := PaginatedData{
		Items: items,
		Pagination: Pagination{
			Page:       page,
			PageSize:   pageSize,
			Total:      total,
			TotalPages: totalPages,
		},
	}
	resp := SuccessResp{
		Code:    0,
		Success: true,
		Data:    data,
		Message: "ok",
		TraceID: traceIDFrom(r),
	}
	return writeJSON(w, http.StatusOK, resp)
}

// HandleError 把 error 转成 JSON 错误响应。
// 用 errors.As 而不是类型断言，是因为 errors.As 能穿透 fmt.Errorf("%w", err) 的多层包装，
// 即使错误被包了好几层，也能找到最里面的 AppError。
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		resp := ErrorResp{
			Code:    appErr.Code,
			Success: false,
			Message: appErr.Message,
			Data:    nil,
			Meta: &ErrorMeta{
				Code:    http.StatusText(appErr.StatusCode),
				Message: appErr.Message,
			},
			TraceID: traceIDFrom(r),
		}
		writeJSON(w, appErr.StatusCode, resp)
		return
	}

	// 非 AppError 的错误一律返回 500，避免泄露内部实现细节给客户端。
	// 同时记录日志，方便排查。用 ErrorContext 而不是 Error，是为了关联请求的 traceId。
	slog.ErrorContext(r.Context(), "unhandled error", "error", err)
	resp := ErrorResp{
		Code:    apperr.ErrCodeInternal,
		Success: false,
		Message: "internal server error",
		Data:    nil,
		TraceID: traceIDFrom(r),
	}
	writeJSON(w, http.StatusInternalServerError, resp)
}

func writeJSON(w http.ResponseWriter, status int, v any) error {
	// 必须在 WriteHeader 之前调用 Header().Set()，否则 header 不生效（Go net/http 的坑）
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// 用自定义类型做 context key，避免和其他包的 string key 冲突。
// 如果直接用 string("traceId")，两个不同包用同样的字符串就会互相覆盖。
type contextKey string

const traceIDKey contextKey = "traceId"

func traceIDFrom(r *http.Request) string {
	if id, ok := r.Context().Value(traceIDKey).(string); ok {
		return id
	}
	return ""
}

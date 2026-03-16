package response

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"go-backend/internal/apperr"
)

// SuccessResp 和 ErrorResp 使用统一的信封格式，与 TypeScript 版 API 契约完全一致。
// SuccessResp and ErrorResp use a unified envelope format, matching the TypeScript API contract exactly.
//
// 为什么要统一格式？前端只需一个拦截器处理所有响应：success=true 走正常逻辑，false 走错误逻辑。
// Why a unified format? The frontend needs just one interceptor: success=true → happy path, false → error path.
type SuccessResp struct {
	Code    int    `json:"code"`    // 业务码，成功时为 0 / Business code, 0 on success
	Success bool   `json:"success"` // 是否成功 / Whether the request succeeded
	Data    any    `json:"data"`    // 响应数据 / Response payload
	Message string `json:"message"` // 简短描述 / Short description
	TraceID string `json:"traceId"` // 请求追踪 ID / Request trace ID for debugging
}

// ErrorMeta 提供开发者级别的错误详情，和 ErrorResp.Message（用户级）分离。
// ErrorMeta provides developer-level error details, separated from ErrorResp.Message (user-facing).
//
// 为什么分两层？用户看到"找不到商品"就够了，开发者还需要看到 "Not Found" 状态文本来排查问题。
// Why two layers? Users just need "product not found"; developers also need "Not Found" status text for debugging.
type ErrorMeta struct {
	Code    string `json:"code"`    // HTTP 状态文本（如 "Not Found"）/ HTTP status text (e.g. "Not Found")
	Message string `json:"message"` // 开发者可读的详细信息 / Developer-readable details
}

// ErrorResp 是统一的错误响应信封。
// ErrorResp is the standard error response envelope.
type ErrorResp struct {
	Code    int        `json:"code"`
	Success bool       `json:"success"`
	Message string     `json:"message"`
	Data    any        `json:"data"`
	Meta    *ErrorMeta `json:"meta,omitempty"` // 指针+omitempty：nil 时 JSON 中不输出此字段，保持响应干净 / pointer+omitempty: omitted from JSON when nil, keeping the response clean
	TraceID string     `json:"traceId"`
}

// Pagination 分页元数据。
// Pagination holds paging metadata.
type Pagination struct {
	Page       int `json:"page"`       // 当前页码 / Current page number
	PageSize   int `json:"pageSize"`   // 每页条数 / Items per page
	Total      int `json:"total"`      // 总条数 / Total item count
	TotalPages int `json:"totalPages"` // 总页数 / Total page count
}

// PaginatedData 将列表数据和分页信息包在一起。
// PaginatedData wraps a list of items with pagination info.
type PaginatedData struct {
	Items      any        `json:"items"`      // 当前页的数据列表 / List of items for the current page
	Pagination Pagination `json:"pagination"` // 分页元数据 / Paging metadata
}

// Success 写入 JSON 成功响应。
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

// Paginated 写入带分页信息的 JSON 成功响应。
// Paginated writes a paginated JSON success response.
func Paginated(w http.ResponseWriter, r *http.Request, items any, total, page, pageSize int) error {
	totalPages := 0
	if pageSize > 0 {
		// 整数向上取整：等价于 math.Ceil(total/pageSize)，但无需浮点运算。
		// Integer ceiling division: equivalent to math.Ceil(total/pageSize) without floating-point math.
		totalPages = (total + pageSize - 1) / pageSize
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
// HandleError converts an error into a JSON error response.
//
// 用 errors.As 而不是类型断言，是因为 errors.As 能穿透 fmt.Errorf("%w", err) 的多层包装，
// 即使错误被包了好几层，也能找到最里面的 AppError。
// We use errors.As instead of type assertion because errors.As can unwrap multiple layers of
// fmt.Errorf("%w", err) wrapping, finding the inner AppError no matter how deeply nested.
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
	// Non-AppError errors always return 500 to avoid leaking internal details to the client.
	//
	// 同时记录日志，方便排查。用 ErrorContext 而不是 Error，是为了关联请求的 traceId。
	// Also log the error for debugging. We use ErrorContext instead of Error to correlate with the request's traceId.
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

// writeJSON 将结构体序列化为 JSON 并写入 HTTP 响应。
// writeJSON serializes a struct to JSON and writes it to the HTTP response.
func writeJSON(w http.ResponseWriter, status int, v any) error {
	// 必须在 WriteHeader 之前调用 Header().Set()，否则 header 不生效（Go net/http 的坑）。
	// Must call Header().Set() BEFORE WriteHeader(), otherwise the header won't take effect (a Go net/http gotcha).
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

// 用自定义类型做 context key，避免和其他包的 string key 冲突。
// Use a custom type for context keys to avoid collisions with string keys from other packages.
//
// 如果直接用 string("traceId")，两个不同包用同样的字符串就会互相覆盖。
// If you use plain string("traceId"), two different packages with the same string would overwrite each other.
type contextKey string

const traceIDKey contextKey = "traceId"

// traceIDFrom 从请求的 context 中提取 traceId。
// traceIDFrom extracts the traceId from the request's context.
func traceIDFrom(r *http.Request) string {
	// 类型断言 .(string)：从 any 类型的 context value 中安全提取 string。
	// Type assertion .(string): safely extracts a string from the any-typed context value.
	// ok=false 表示值不存在或类型不匹配。
	// ok=false means the value doesn't exist or the type doesn't match.
	if id, ok := r.Context().Value(traceIDKey).(string); ok {
		return id
	}
	return ""
}

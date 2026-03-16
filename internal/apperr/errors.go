package apperr

import (
	"fmt"
	"net/http"
)

// AppError 同时携带业务错误码（Code）和 HTTP 状态码（StatusCode）。
// AppError carries both a business error code (Code) and an HTTP status code (StatusCode).
//
// 为什么要两个码？HTTP 状态码给网络层用（浏览器/代理），业务码给前端逻辑用。
// Why two codes? HTTP status is for the network layer (browsers/proxies); business code is for frontend logic.
//
// 比如"邮箱已存在"和"SKU 已存在"都是 409 Conflict，但业务码不同，前端需要区分。
// E.g. "email exists" and "SKU exists" are both 409 Conflict, but different business codes let the frontend distinguish them.
//
// StatusCode 标记 json:"-" 是因为它已经在 HTTP 响应头中，不需要重复出现在 body 里。
// StatusCode is tagged json:"-" because it's already in the HTTP response header — no need to repeat it in the body.
type AppError struct {
	Code       int    `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

// Error 实现 error 接口。只要有这个方法，*AppError 就能当 error 用。
// Error implements the error interface. With this method, *AppError satisfies the error interface.
//
// 这是 Go 的隐式接口——不需要 implements 声明，只要方法签名匹配就自动满足。
// This is Go's implicit interface — no "implements" declaration needed; matching the method signature is enough.
func (e *AppError) Error() string {
	return e.Message
}

// New 创建一个自定义 AppError，指定业务码、HTTP 状态码和消息。
// New creates a custom AppError with the given business code, HTTP status, and message.
func New(code int, statusCode int, message string) *AppError {
	return &AppError{
		Code:       code,
		StatusCode: statusCode,
		Message:    message,
	}
}

// NewNotFound 创建 404 资源未找到错误。message 自动包含资源类型和标识符，方便排查。
// NewNotFound creates a 404 not-found error. The message auto-includes resource type and identifier for easier debugging.
func NewNotFound(resource, identifier string) *AppError {
	return &AppError{
		Code:       ErrCodeNotFound,
		StatusCode: http.StatusNotFound,
		Message:    fmt.Sprintf("%s not found: %s", resource, identifier),
	}
}

// NewBadRequest 创建 400 请求格式错误（JSON 解析失败、缺少必填字段等）。
// NewBadRequest creates a 400 error for malformed requests (bad JSON, missing required fields, etc.).
func NewBadRequest(message string) *AppError {
	return &AppError{
		Code:       ErrCodeBadRequest,
		StatusCode: http.StatusBadRequest,
		Message:    message,
	}
}

// NewUnauthorized 创建 401 未认证错误（未登录、token 无效/过期）。
// NewUnauthorized creates a 401 error for unauthenticated requests (not logged in, invalid/expired token).
func NewUnauthorized(message string) *AppError {
	return &AppError{
		Code:       ErrCodeUnauthorized,
		StatusCode: http.StatusUnauthorized,
		Message:    message,
	}
}

// NewForbidden 创建 403 无权限错误（已登录但没有操作权限）。
// NewForbidden creates a 403 error for unauthorized actions (logged in but lacking permission).
func NewForbidden(message string) *AppError {
	return &AppError{
		Code:       ErrCodeForbidden,
		StatusCode: http.StatusForbidden,
		Message:    message,
	}
}

// NewConflict 创建 409 冲突错误（资源已存在，如邮箱重复注册）。
// NewConflict creates a 409 conflict error (resource already exists, e.g. duplicate email registration).
func NewConflict(resource, identifier string) *AppError {
	return &AppError{
		Code:       ErrCodeConflict,
		StatusCode: http.StatusConflict,
		Message:    fmt.Sprintf("%s already exists: %s", resource, identifier),
	}
}

// NewValidation 创建 422 校验错误（字段格式不对、值超出范围等业务校验失败）。
// NewValidation creates a 422 validation error (invalid field format, value out of range, etc.).
func NewValidation(message string) *AppError {
	return &AppError{
		Code:       ErrCodeValidation,
		StatusCode: http.StatusUnprocessableEntity,
		Message:    message,
	}
}

// NewInternal 创建 500 内部错误（数据库连接失败、未预期的 panic 等）。
// NewInternal creates a 500 internal server error (DB connection failure, unexpected panic, etc.).
func NewInternal(message string) *AppError {
	return &AppError{
		Code:       ErrCodeInternal,
		StatusCode: http.StatusInternalServerError,
		Message:    message,
	}
}

// NewRateLimited 创建 429 限流错误（请求频率超过限制）。
// NewRateLimited creates a 429 rate-limit error (request frequency exceeded).
func NewRateLimited() *AppError {
	return &AppError{
		Code:       ErrCodeRateLimited,
		StatusCode: http.StatusTooManyRequests,
		Message:    "rate limit exceeded",
	}
}

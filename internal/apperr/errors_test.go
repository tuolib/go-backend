package apperr

import (
	"errors"
	"net/http"
	"testing"
)

// TestAppError_Error 测试 AppError 的 Error() 方法返回正确的消息。
// TestAppError_Error verifies that AppError's Error() method returns the correct message.
func TestAppError_Error(t *testing.T) {
	err := New(ErrCodeNotFound, http.StatusNotFound, "user not found")
	if err.Error() != "user not found" {
		t.Errorf("got %q, want %q", err.Error(), "user not found")
	}
}

// TestAppError_ImplementsError 验证 AppError 实现了 error 接口。
// TestAppError_ImplementsError verifies that AppError implements the error interface.
//
// var err error = NewBadRequest("bad") 这行用接口类型接收，如果 AppError 没实现 error 接口，编译就会报错。
// The line "var err error = ..." assigns to an interface type — a compile error if AppError doesn't implement error.
func TestAppError_ImplementsError(t *testing.T) {
	var err error = NewBadRequest("bad")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
}

// TestAppError_ErrorsAs 测试 errors.As 能正确匹配 *AppError 类型。
// TestAppError_ErrorsAs tests that errors.As correctly matches the *AppError type.
//
// errors.As 是 Go 1.13 引入的错误类型匹配函数，能穿透 %w 包装。
// errors.As is a Go 1.13 error type matching function that can unwrap %w wrapping.
func TestAppError_ErrorsAs(t *testing.T) {
	err := NewNotFound("user", "test@example.com")
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatal("expected errors.As to match *AppError")
	}
	if appErr.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", appErr.StatusCode, http.StatusNotFound)
	}
}

// TestFactories 用表驱动测试验证所有工厂函数生成正确的状态码和业务码。
// TestFactories uses table-driven tests to verify all factory functions produce correct status and business codes.
//
// 表驱动测试是 Go 的惯用模式：把测试用例放在切片里循环执行，新增用例只需加一行。
// Table-driven tests are idiomatic Go: put test cases in a slice and loop — adding a case is just one more line.
func TestFactories(t *testing.T) {
	tests := []struct {
		name       string    // 测试用例名称 / Test case name
		err        *AppError // 要测试的错误 / Error under test
		wantStatus int       // 期望的 HTTP 状态码 / Expected HTTP status code
		wantCode   int       // 期望的业务错误码 / Expected business error code
	}{
		{"NotFound", NewNotFound("user", "x"), http.StatusNotFound, ErrCodeNotFound},
		{"BadRequest", NewBadRequest("bad"), http.StatusBadRequest, ErrCodeBadRequest},
		{"Unauthorized", NewUnauthorized("no auth"), http.StatusUnauthorized, ErrCodeUnauthorized},
		{"Forbidden", NewForbidden("denied"), http.StatusForbidden, ErrCodeForbidden},
		{"Conflict", NewConflict("user", "x"), http.StatusConflict, ErrCodeConflict},
		{"Validation", NewValidation("invalid"), http.StatusUnprocessableEntity, ErrCodeValidation},
		{"Internal", NewInternal("oops"), http.StatusInternalServerError, ErrCodeInternal},
		{"RateLimited", NewRateLimited(), http.StatusTooManyRequests, ErrCodeRateLimited},
	}

	for _, tt := range tests {
		// t.Run 创建子测试，失败时会显示具体是哪个工厂函数出了问题。
		// t.Run creates a subtest — on failure it shows exactly which factory function failed.
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.StatusCode != tt.wantStatus {
				t.Errorf("StatusCode = %d, want %d", tt.err.StatusCode, tt.wantStatus)
			}
			if tt.err.Code != tt.wantCode {
				t.Errorf("Code = %d, want %d", tt.err.Code, tt.wantCode)
			}
		})
	}
}

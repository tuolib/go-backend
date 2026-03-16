package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-backend/internal/apperr"
)

// TestWrap_Success 测试 handler 返回 nil error 时正常响应。
// TestWrap_Success tests that a handler returning nil error produces a normal response.
func TestWrap_Success(t *testing.T) {
	// Wrap 把 AppHandler 转成标准的 http.HandlerFunc。
	// Wrap converts an AppHandler into a standard http.HandlerFunc.
	h := Wrap(func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		return json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// httptest.NewRecorder() 创建一个假的 ResponseWriter，用于捕获 handler 写入的响应。
	// httptest.NewRecorder() creates a fake ResponseWriter to capture the handler's response.
	w := httptest.NewRecorder()
	// httptest.NewRequest() 创建一个假的 HTTP 请求，不需要真实的网络连接。
	// httptest.NewRequest() creates a fake HTTP request — no real network connection needed.
	r := httptest.NewRequest(http.MethodPost, "/test", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// TestWrap_AppError 测试 handler 返回 AppError 时，Wrap 自动将其转为对应的 HTTP 错误响应。
// TestWrap_AppError tests that when a handler returns an AppError, Wrap auto-converts it to the corresponding HTTP error response.
func TestWrap_AppError(t *testing.T) {
	h := Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return apperr.NewNotFound("item", "123") // 直接返回错误，Wrap 会处理 / Just return the error — Wrap handles it
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

// TestWrap_NilError 测试 handler 返回 nil 时不触发错误处理。
// TestWrap_NilError tests that returning nil doesn't trigger error handling.
func TestWrap_NilError(t *testing.T) {
	h := Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return nil // 没有错误，Wrap 什么也不做 / No error — Wrap does nothing
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)
	h.ServeHTTP(w, r)

	// handler 没有写任何响应，默认 200
	// Handler wrote no response — defaults to 200
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

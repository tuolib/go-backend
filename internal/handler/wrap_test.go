package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go-backend/internal/apperr"
)

func TestWrap_Success(t *testing.T) {
	h := Wrap(func(w http.ResponseWriter, r *http.Request) error {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		return json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWrap_AppError(t *testing.T) {
	h := Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return apperr.NewNotFound("item", "123")
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestWrap_NilError(t *testing.T) {
	h := Wrap(func(w http.ResponseWriter, r *http.Request) error {
		return nil
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/test", nil)
	h.ServeHTTP(w, r)

	// No error, no response written by Wrap
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

package apperr

import (
	"errors"
	"net/http"
	"testing"
)

func TestAppError_Error(t *testing.T) {
	err := New(ErrCodeNotFound, http.StatusNotFound, "user not found")
	if err.Error() != "user not found" {
		t.Errorf("got %q, want %q", err.Error(), "user not found")
	}
}

func TestAppError_ImplementsError(t *testing.T) {
	var err error = NewBadRequest("bad")
	if err == nil {
		t.Fatal("expected non-nil error")
	}
}

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

func TestFactories(t *testing.T) {
	tests := []struct {
		name       string
		err        *AppError
		wantStatus int
		wantCode   int
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

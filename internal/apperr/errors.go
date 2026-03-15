package apperr

import (
	"fmt"
	"net/http"
)

// AppError represents a structured application error with HTTP status code.
type AppError struct {
	Code       int    `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

func (e *AppError) Error() string {
	return e.Message
}

// New creates a new AppError.
func New(code int, statusCode int, message string) *AppError {
	return &AppError{
		Code:       code,
		StatusCode: statusCode,
		Message:    message,
	}
}

// NewNotFound creates a 404 not found error.
func NewNotFound(resource, identifier string) *AppError {
	return &AppError{
		Code:       ErrCodeNotFound,
		StatusCode: http.StatusNotFound,
		Message:    fmt.Sprintf("%s not found: %s", resource, identifier),
	}
}

// NewBadRequest creates a 400 bad request error.
func NewBadRequest(message string) *AppError {
	return &AppError{
		Code:       ErrCodeBadRequest,
		StatusCode: http.StatusBadRequest,
		Message:    message,
	}
}

// NewUnauthorized creates a 401 unauthorized error.
func NewUnauthorized(message string) *AppError {
	return &AppError{
		Code:       ErrCodeUnauthorized,
		StatusCode: http.StatusUnauthorized,
		Message:    message,
	}
}

// NewForbidden creates a 403 forbidden error.
func NewForbidden(message string) *AppError {
	return &AppError{
		Code:       ErrCodeForbidden,
		StatusCode: http.StatusForbidden,
		Message:    message,
	}
}

// NewConflict creates a 409 conflict error.
func NewConflict(resource, identifier string) *AppError {
	return &AppError{
		Code:       ErrCodeConflict,
		StatusCode: http.StatusConflict,
		Message:    fmt.Sprintf("%s already exists: %s", resource, identifier),
	}
}

// NewValidation creates a 422 validation error.
func NewValidation(message string) *AppError {
	return &AppError{
		Code:       ErrCodeValidation,
		StatusCode: http.StatusUnprocessableEntity,
		Message:    message,
	}
}

// NewInternal creates a 500 internal server error.
func NewInternal(message string) *AppError {
	return &AppError{
		Code:       ErrCodeInternal,
		StatusCode: http.StatusInternalServerError,
		Message:    message,
	}
}

// NewRateLimited creates a 429 rate limit error.
func NewRateLimited() *AppError {
	return &AppError{
		Code:       ErrCodeRateLimited,
		StatusCode: http.StatusTooManyRequests,
		Message:    "rate limit exceeded",
	}
}

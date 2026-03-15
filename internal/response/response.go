package response

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"go-backend/internal/apperr"
)

// SuccessResp is the standard success response envelope.
type SuccessResp struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Data    any    `json:"data"`
	Message string `json:"message"`
	TraceID string `json:"traceId"`
}

// ErrorMeta provides developer-facing error details.
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
	Meta    *ErrorMeta `json:"meta,omitempty"`
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

// HandleError converts an error to a JSON error response.
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

	// Unknown error → 500
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
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	return json.NewEncoder(w).Encode(v)
}

type contextKey string

const traceIDKey contextKey = "traceId"

func traceIDFrom(r *http.Request) string {
	if id, ok := r.Context().Value(traceIDKey).(string); ok {
		return id
	}
	return ""
}

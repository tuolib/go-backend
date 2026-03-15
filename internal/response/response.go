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

// ErrorResp is the standard error response envelope.
type ErrorResp struct {
	Code    int    `json:"code"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	TraceID string `json:"traceId"`
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

// HandleError converts an error to a JSON error response.
func HandleError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *apperr.AppError
	if errors.As(err, &appErr) {
		resp := ErrorResp{
			Code:    appErr.Code,
			Success: false,
			Message: appErr.Message,
			Data:    nil,
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

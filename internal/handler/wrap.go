package handler

import (
	"net/http"

	"go-backend/internal/response"
)

// AppHandler is an HTTP handler that returns an error.
// Errors are converted to JSON responses by Wrap.
type AppHandler func(w http.ResponseWriter, r *http.Request) error

// Wrap converts an AppHandler into a standard http.HandlerFunc.
// If the handler returns an error, it is passed to response.HandleError
// for unified error response formatting.
func Wrap(h AppHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			response.HandleError(w, r, err)
		}
	}
}

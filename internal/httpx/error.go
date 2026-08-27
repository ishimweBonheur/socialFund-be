package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *Error) Error() string { return e.Code }
func NewError(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

var (
	ErrValidation   = NewError(400, "VALIDATION_ERROR", "The request contains invalid data")
	ErrUnauthorized = NewError(401, "UNAUTHORIZED", "Authentication is required")
	ErrForbidden    = NewError(403, "FORBIDDEN", "You do not have permission to perform this action")
	ErrInternal     = NewError(500, "INTERNAL_SERVER_ERROR", "An unexpected error occurred")
)

func WriteError(w http.ResponseWriter, err error) {
	apiErr, ok := err.(*Error)
	if !ok {
		apiErr = ErrInternal
	}
	WriteJSON(w, apiErr.Status, map[string]any{"error": apiErr})
}
func WriteInternal(w http.ResponseWriter, r *http.Request, logger *slog.Logger, operation string, err error) {
	logger.ErrorContext(r.Context(), "request failed", "request_id", RequestID(r.Context()), "operation", operation, "error", err)
	WriteError(w, ErrInternal)
}
func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

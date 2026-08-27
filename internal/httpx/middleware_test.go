package httpx_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"socialfund/internal/httpx"
)

func TestRequestLoggingIncludesSafeCorrelationFields(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := httpx.RequestIDMiddleware(httpx.LoggingMiddleware(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if httpx.RequestID(r.Context()) == "" {
			t.Error("request ID missing from context")
		}
		w.WriteHeader(http.StatusCreated)
	})))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", nil)
	request.Header.Set("Authorization", "Bearer secret-token-that-must-not-be-logged")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	requestID := response.Header().Get("X-Request-ID")
	if requestID == "" {
		t.Fatal("response request ID missing")
	}
	output := logs.String()
	for _, value := range []string{"request_id", "method", "path", "status", "duration_ms", requestID, "/api/v1/admin/users"} {
		if !strings.Contains(output, value) {
			t.Fatalf("log missing %q: %s", value, output)
		}
	}
	if strings.Contains(output, "secret-token") || strings.Contains(output, "Authorization") {
		t.Fatalf("sensitive header logged: %s", output)
	}
}

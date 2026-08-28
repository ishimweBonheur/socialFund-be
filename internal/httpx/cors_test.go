package httpx_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"socialfund/internal/httpx"
)

func TestCORSHandlesDevelopmentPreflight(t *testing.T) {
	called := false
	handler := httpx.CORS("http://localhost:5173")(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/google", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusNoContent)
	}
	if called {
		t.Fatal("preflight request reached the application handler")
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allowed origin = %q", got)
	}
}

func TestCORSRejectsUnconfiguredOrigin(t *testing.T) {
	handler := httpx.CORS("http://localhost:5173")(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}))
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/auth/google", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	res := httptest.NewRecorder()

	handler.ServeHTTP(res, req)

	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusMethodNotAllowed)
	}
	if got := res.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("unexpected allowed origin %q", got)
	}
}

package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimiterReturns429AndRetryAfter(t *testing.T) {
	lim := NewRateLimiter(1)
	handler := lim.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest("GET", "/", nil))
	if first.Code != 204 {
		t.Fatalf("first=%d", first.Code)
	}
	second := httptest.NewRecorder()
	handler.ServeHTTP(second, httptest.NewRequest("GET", "/", nil))
	if second.Code != 429 || second.Header().Get("Retry-After") == "" {
		t.Fatalf("second=%d retry=%q", second.Code, second.Header().Get("Retry-After"))
	}
}

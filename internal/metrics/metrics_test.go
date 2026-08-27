package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMiddlewareCountsRequest(t *testing.T) {
	r := New(nil)
	wrapped := r.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(201) }))
	wrapped.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/test", nil))
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.requests) != 1 {
		t.Fatalf("requests=%d", len(r.requests))
	}
	for key := range r.requests {
		if !strings.Contains(key, "POST") || strings.Contains(key, "secret") {
			t.Fatalf("unsafe metric key %q", key)
		}
	}
}

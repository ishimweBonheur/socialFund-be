package audit

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestMetadataMiddlewareCapturesClient(t *testing.T) {
	var captured AuditLog
	handler := RequestMetadataMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = enrichFromContext(r.Context(), AuditLog{})
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/contributions/id/approve", nil)
	req.RemoteAddr = "10.0.0.8:4321"
	req.Header.Set("User-Agent", "SocialFund-Web/1.0")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if captured.IPAddress == nil || *captured.IPAddress != "10.0.0.8" {
		t.Fatalf("expected client IP, got %v", captured.IPAddress)
	}
	if captured.UserAgent == nil || *captured.UserAgent != "SocialFund-Web/1.0" {
		t.Fatalf("expected user agent, got %v", captured.UserAgent)
	}
}

func TestRequestMetadataMiddlewareUsesForwardedClient(t *testing.T) {
	var captured AuditLog
	handler := RequestMetadataMiddleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		captured = enrichFromContext(r.Context(), AuditLog{})
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 10.0.0.2")
	handler.ServeHTTP(httptest.NewRecorder(), req)

	if captured.IPAddress == nil || *captured.IPAddress != "203.0.113.9" {
		t.Fatalf("expected forwarded client IP, got %v", captured.IPAddress)
	}
}

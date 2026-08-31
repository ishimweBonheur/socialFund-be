package httpx

import (
	"bufio"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func (r *responseRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
func LoggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			rec := &responseRecorder{ResponseWriter: w, status: 200}
			next.ServeHTTP(rec, r)
			attrs := []any{"service", "social-fund", "request_id", RequestID(r.Context()), "method", r.Method, "path", r.URL.Path, "status", rec.status, "duration_ms", time.Since(started).Milliseconds(), "remote_ip", r.RemoteAddr}
			if identity, ok := IdentityFrom(r.Context()); ok {
				attrs = append(attrs, "authenticated_user_id", identity.UserID)
			}
			logger.InfoContext(r.Context(), "http request", attrs...)
		})
	}
}

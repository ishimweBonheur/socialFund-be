package audit

import (
	"context"
	"net"
	"net/http"
	"strings"
)

type requestMetadata struct {
	ipAddress string
	userAgent string
}

type requestMetadataKey struct{}

// RequestMetadataMiddleware records request information in the context so every
// audit entry created during the request receives the same actor metadata.
func RequestMetadataMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metadata := requestMetadata{
			ipAddress: clientIP(r),
			userAgent: strings.TrimSpace(r.UserAgent()),
		}
		ctx := context.WithValue(r.Context(), requestMetadataKey{}, metadata)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func enrichFromContext(ctx context.Context, entry AuditLog) AuditLog {
	metadata, _ := ctx.Value(requestMetadataKey{}).(requestMetadata)
	if entry.IPAddress == nil && metadata.ipAddress != "" {
		entry.IPAddress = &metadata.ipAddress
	}
	if entry.UserAgent == nil && metadata.userAgent != "" {
		entry.UserAgent = &metadata.userAgent
	}
	return entry
}

func clientIP(r *http.Request) string {
	// These headers are set by the deployment proxy. The first X-Forwarded-For
	// value is the original client when proxies append their own addresses.
	if forwarded := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		if first, _, ok := strings.Cut(forwarded, ","); ok {
			return strings.TrimSpace(first)
		}
		return forwarded
	}
	if realIP := strings.TrimSpace(r.Header.Get("X-Real-IP")); realIP != "" {
		return realIP
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

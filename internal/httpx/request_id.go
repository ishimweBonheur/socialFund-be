package httpx

import (
	"context"
	"github.com/google/uuid"
	"net/http"
)

type contextKey string

const (
	requestIDKey contextKey = "request_id"
	identityKey  contextKey = "identity"
)

type Identity struct {
	UserID uuid.UUID
	Role   string
}

func RequestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}
func IdentityFrom(ctx context.Context) (Identity, bool) {
	value, ok := ctx.Value(identityKey).(Identity)
	return value, ok
}
func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityKey, identity)
}
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if _, err := uuid.Parse(id); err != nil {
			id = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

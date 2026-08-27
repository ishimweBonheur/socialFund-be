package auth

import (
	"net/http"
	"socialfund/internal/httpx"
	"strings"
)

func Authenticate(tokens *TokenManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				httpx.WriteError(w, httpx.ErrUnauthorized)
				return
			}
			identity, err := tokens.Verify(parts[1])
			if err != nil {
				httpx.WriteError(w, httpx.ErrUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(httpx.WithIdentity(r.Context(), identity)))
		})
	}
}
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identity, ok := httpx.IdentityFrom(r.Context())
		if !ok {
			httpx.WriteError(w, httpx.ErrUnauthorized)
			return
		}
		if identity.Role != "ADMIN" {
			httpx.WriteError(w, httpx.ErrForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

package auth_test

import (
	"net/http"
	"net/http/httptest"
	"socialfund/internal/auth"
	"socialfund/internal/httpx"
	"testing"
)

func TestMemberCannotAccessAdminRoute(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", nil)
	request = request.WithContext(httpx.WithIdentity(request.Context(), httpx.Identity{Role: "MEMBER"}))
	response := httptest.NewRecorder()
	auth.RequireAdmin(next).ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d", response.Code)
	}
	if response.Body.String() == "" {
		t.Fatal("error response missing")
	}
}

package database

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"socialfund/internal/httpx"
	"time"
)

func HealthHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			httpx.WriteError(w, httpx.NewError(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Service is unavailable"))
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

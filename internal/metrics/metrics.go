package metrics

import (
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Registry struct {
	mu       sync.Mutex
	requests map[string]uint64
	duration map[string]float64
	active   int64
	pool     *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Registry {
	return &Registry{requests: map[string]uint64{}, duration: map[string]float64{}, pool: pool}
}

type capture struct {
	http.ResponseWriter
	status int
}

func (c *capture) WriteHeader(s int) { c.status = s; c.ResponseWriter.WriteHeader(s) }
func (r *Registry) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()
		r.mu.Lock()
		r.active++
		r.mu.Unlock()
		c := &capture{ResponseWriter: w, status: 200}
		next.ServeHTTP(c, req)
		route := chi.RouteContext(req.Context()).RoutePattern()
		if route == "" {
			route = "unknown"
		}
		key := req.Method + "|" + route + "|" + strconv.Itoa(c.status)
		r.mu.Lock()
		r.active--
		r.requests[key]++
		r.duration[key] += time.Since(start).Seconds()
		r.mu.Unlock()
	})
}
func (r *Registry) Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	r.mu.Lock()
	defer r.mu.Unlock()
	fmt.Fprintf(w, "# TYPE socialfund_http_requests_total counter\n")
	for key, n := range r.requests {
		parts := strings.SplitN(key, "|", 3)
		fmt.Fprintf(w, "socialfund_http_requests_total{method=%q,route=%q,status=%q} %d\n", parts[0], parts[1], parts[2], n)
	}
	fmt.Fprintf(w, "# TYPE socialfund_http_request_duration_seconds_total counter\n")
	for key, n := range r.duration {
		parts := strings.SplitN(key, "|", 3)
		fmt.Fprintf(w, "socialfund_http_request_duration_seconds_total{method=%q,route=%q,status=%q} %f\n", parts[0], parts[1], parts[2], n)
	}
	fmt.Fprintf(w, "# TYPE socialfund_http_active_requests gauge\nsocialfund_http_active_requests %d\n", r.active)
	st := r.pool.Stat()
	fmt.Fprintf(w, "# TYPE socialfund_db_connections gauge\nsocialfund_db_connections{state=\"total\"} %d\nsocialfund_db_connections{state=\"acquired\"} %d\nsocialfund_db_connections{state=\"idle\"} %d\n", st.TotalConns(), st.AcquiredConns(), st.IdleConns())
}

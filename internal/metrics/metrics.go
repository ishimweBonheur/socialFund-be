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

func (c *capture) WriteHeader(s int) {
	c.status = s
	c.ResponseWriter.WriteHeader(s)
}
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
func (r *Registry) Handler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	r.mu.Lock()
	defer r.mu.Unlock()
	lines := make([]string, 0)
	for key, n := range r.requests {
		parts := strings.SplitN(key, "|", 3)
		lines = append(lines, fmt.Sprintf("socialfund_http_requests_total{method=%q,route=%q,status=%q} %d", parts[0], parts[1], parts[2], n))
	}
	for key, n := range r.duration {
		parts := strings.SplitN(key, "|", 3)
		lines = append(lines, fmt.Sprintf("socialfund_http_request_duration_seconds_total{method=%q,route=%q,status=%q} %f", parts[0], parts[1], parts[2], n))
	}
	lines = append(lines, fmt.Sprintf("socialfund_http_active_requests %d", r.active))
	st := r.pool.Stat()
	lines = append(lines, fmt.Sprintf("socialfund_db_connections{state=\"total\"} %d", st.TotalConns()), fmt.Sprintf("socialfund_db_connections{state=\"acquired\"} %d", st.AcquiredConns()), fmt.Sprintf("socialfund_db_connections{state=\"idle\"} %d", st.IdleConns()))
	search := strings.ToLower(strings.TrimSpace(req.URL.Query().Get("search")))
	if search != "" {
		filtered := lines[:0]
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), search) {
				filtered = append(filtered, line)
			}
		}
		lines = filtered
	}
	total := len(lines)
	limit, _ := strconv.Atoi(req.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(req.URL.Query().Get("offset"))
	if limit > 0 {
		if offset < 0 {
			offset = 0
		}
		if offset > total {
			offset = total
		}
		end := offset + limit
		if end > total {
			end = total
		}
		lines = lines[offset:end]
	}
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	w.Header().Set("Access-Control-Expose-Headers", "X-Total-Count")
	fmt.Fprintln(w, strings.Join(lines, "\n"))
}

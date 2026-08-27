package httpx

import (
	"golang.org/x/time/rate"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type RateLimiter struct {
	mu    sync.Mutex
	items map[string]*rate.Limiter
	rpm   int
}

func NewRateLimiter(rpm int) *RateLimiter {
	return &RateLimiter{items: make(map[string]*rate.Limiter), rpm: rpm}
}
func (l *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := "ip:" + clientIP(r)
		if actor, ok := IdentityFrom(r.Context()); ok {
			key = "user:" + actor.UserID.String()
		}
		l.mu.Lock()
		lim := l.items[key]
		if lim == nil {
			perSecond := rate.Limit(float64(l.rpm) / 60)
			lim = rate.NewLimiter(perSecond, max(1, l.rpm/6))
			l.items[key] = lim
		}
		allowed := lim.Allow()
		l.mu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", strconv.Itoa(max(1, 60/l.rpm)))
			WriteError(w, NewError(429, "RATE_LIMIT_EXCEEDED", "Too many requests. Please try again later."))
			return
		}
		next.ServeHTTP(w, r)
	})
}
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
func (l *RateLimiter) Cleanup(ctxDone <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctxDone:
			return
		case <-ticker.C:
			l.mu.Lock()
			if len(l.items) > 10000 {
				l.items = make(map[string]*rate.Limiter)
			}
			l.mu.Unlock()
		}
	}
}

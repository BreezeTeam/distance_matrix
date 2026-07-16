package middleware

import (
	"net/http"
	"sync"
	"time"
)

type tenantLimiter struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	counters map[string]*windowCount
}

type windowCount struct {
	start time.Time
	count int
}

// TenantRateLimit returns middleware limiting requests per tenant per second.
func TenantRateLimit(limit int) func(http.HandlerFunc) http.HandlerFunc {
	if limit <= 0 {
		limit = 50
	}
	lim := &tenantLimiter{
		limit:    limit,
		window:   time.Second,
		counters: make(map[string]*windowCount),
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			tenant := r.Header.Get("X-Tenant-Id")
			if tenant == "" {
				tenant = "default"
			}
			if !lim.allow(tenant) {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"code":429,"msg":"RATE_LIMIT"}`))
				return
			}
			next(w, r)
		}
	}
}

func (l *tenantLimiter) allow(tenant string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	wc, ok := l.counters[tenant]
	if !ok || now.Sub(wc.start) >= l.window {
		l.counters[tenant] = &windowCount{start: now, count: 1}
		return true
	}
	if wc.count >= l.limit {
		return false
	}
	wc.count++
	return true
}

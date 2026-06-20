package router

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// ipRateLimiter enforces a token-bucket rate limit keyed per client IP.
//
// The pre-auth endpoints (/auth/callback, /auth/refresh, /claim) run before any
// tenant context exists, so they cannot use the per-tenant limiter. Using a
// single process-wide bucket (the previous behaviour) let one abusive IP lock
// out logins for every user (DoS). Keying per source IP isolates abusers while
// leaving legitimate clients unaffected (SEC-3).
type ipRateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	rps      rate.Limit
	burst    int
	ttl      time.Duration // idle entries older than ttl are evicted
	maxSize  int           // hard cap on tracked IPs (bounds memory)
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// newIPRateLimiter creates a per-IP limiter allowing rps requests/sec with the
// given burst. Idle IPs are evicted after 10 minutes; the map is capped to
// avoid unbounded growth under a spoofed-IP flood.
func newIPRateLimiter(rps float64, burst int) *ipRateLimiter {
	return &ipRateLimiter{
		visitors: make(map[string]*visitor),
		rps:      rate.Limit(rps),
		burst:    burst,
		ttl:      10 * time.Minute,
		maxSize:  50000,
	}
}

// limiterFor returns the token bucket for ip, creating it on first use and
// opportunistically evicting stale entries to bound memory.
func (l *ipRateLimiter) limiterFor(ip string, now time.Time) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	if v, ok := l.visitors[ip]; ok {
		v.lastSeen = now
		return v.limiter
	}

	// Opportunistic cleanup when the map grows large.
	if len(l.visitors) >= l.maxSize {
		for k, v := range l.visitors {
			if now.Sub(v.lastSeen) > l.ttl {
				delete(l.visitors, k)
			}
		}
	}

	lim := rate.NewLimiter(l.rps, l.burst)
	l.visitors[ip] = &visitor{limiter: lim, lastSeen: now}
	return lim
}

// middleware returns a chi-compatible middleware enforcing the per-IP limit.
func (l *ipRateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.limiterFor(clientIP(r), time.Now()).Allow() {
			w.Header().Set("Retry-After", "1")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"code":"rate_limited","message":"too many requests"}}`)) //nolint:errcheck
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP extracts the originating client IP. Behind the reverse proxy (Caddy)
// the real client is the left-most entry of X-Forwarded-For; otherwise fall back
// to the connection's RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if first := strings.TrimSpace(strings.Split(xff, ",")[0]); first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

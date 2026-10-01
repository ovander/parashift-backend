package router

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// SEC-3 core guarantee: one abusive IP must not throttle a different IP.
func TestIPRateLimiter_PerIPIsolation(t *testing.T) {
	// 1 token, no refill within the test window → burst of exactly 1 per IP.
	lim := newIPRateLimiter(0.0001, 1)
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := lim.middleware(next)

	call := func(ip string) int {
		// Behind Caddy: the peer is loopback and Caddy appends the browser.
		req := httptest.NewRequest(http.MethodPost, "/auth/callback", nil)
		req.RemoteAddr = "127.0.0.1:5000"
		req.Header.Set("X-Forwarded-For", ip)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	// Attacker IP: first request OK, second exhausts the bucket → 429.
	assert.Equal(t, http.StatusOK, call("1.1.1.1"))
	assert.Equal(t, http.StatusTooManyRequests, call("1.1.1.1"))

	// A different IP is unaffected by the attacker — still allowed.
	assert.Equal(t, http.StatusOK, call("2.2.2.2"),
		"a second IP must not be throttled by the first IP's abuse")
}

// S5: a browser cannot reset its bucket by writing X-Forwarded-For; only the
// entry Caddy appends (rightmost, from a loopback peer) counts.
func TestIPRateLimiter_SpoofedForwardedForDoesNotEscape(t *testing.T) {
	lim := newIPRateLimiter(0.0001, 1)
	h := lim.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))

	call := func(spoofed string) int {
		req := httptest.NewRequest(http.MethodPost, "/auth/callback", nil)
		req.RemoteAddr = "127.0.0.1:5000"
		req.Header.Set("X-Forwarded-For", spoofed+", 203.0.113.7")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	assert.Equal(t, http.StatusOK, call("1.1.1.1"))
	assert.Equal(t, http.StatusTooManyRequests, call("9.9.9.9"),
		"a new left-most X-Forwarded-For value must not open a new bucket")
}

// Stale entries are evicted so the visitor map cannot grow unbounded.
func TestIPRateLimiter_EvictsStaleEntries(t *testing.T) {
	lim := newIPRateLimiter(1, 1)
	lim.maxSize = 2
	lim.ttl = time.Millisecond

	base := time.Now()
	lim.limiterFor("a", base)
	lim.limiterFor("b", base)
	// "a" and "b" are now stale relative to a later timestamp; adding "c" past
	// maxSize triggers eviction of expired entries.
	lim.limiterFor("c", base.Add(time.Second))

	lim.mu.Lock()
	n := len(lim.visitors)
	lim.mu.Unlock()
	assert.LessOrEqual(t, n, 2, "stale visitors should be evicted to bound memory")
}

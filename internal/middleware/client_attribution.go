package middleware

import (
	"net"
	"net/http"
	"strings"

	"github.com/ovander/backendkit/bff"
)

// SocrateClientAttribution puts the browser's address and User-Agent on the
// request context (bff.WithClientAttribution), so the socrate.Client calls made
// on the user's behalf (code exchange, refresh, revocation) send them to
// Socrate: backendkit sets X-Forwarded-For to exactly that one address,
// replacing any value, and removes X-Real-IP. Socrate trusts X-Forwarded-For
// from the apps VPS, so it audits, rate-limits and blocks by the browser
// rather than by this server (compat report row S5, Ascenda lesson 8).
//
// The address is AttributionIP's, never a header copied from the browser.
func SocrateClientAttribution() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, bff.WithClientAttribution(r, AttributionIP(r)))
		})
	}
}

// AttributionIP resolves the browser's address. It trusts X-Forwarded-For only
// when the TCP peer is loopback, i.e. Caddy on this host, and then takes the
// rightmost entry that is not loopback (the one Caddy appended; entries to its
// left came from the browser), or nothing when that entry does not parse. From
// any other peer it returns the peer's own address and ignores the headers.
// X-Real-IP is never read. A loopback peer without a forwarded address yields
// "": the request came from this host, not from a browser.
func AttributionIP(r *http.Request) string {
	peer := peerIP(r)
	if peer == nil {
		return ""
	}
	if !peer.IsLoopback() {
		return peer.String()
	}
	// Several X-Forwarded-For headers read as one list, rightmost last. Walk
	// from the right past loopback hops; the first other entry is the one Caddy
	// appended. If it does not parse, stop: never fall back to an entry further
	// left, which the browser wrote.
	entries := strings.Split(strings.Join(r.Header.Values("X-Forwarded-For"), ","), ",")
	for i := len(entries) - 1; i >= 0; i-- {
		entry := strings.TrimSpace(entries[i])
		if entry == "" {
			continue
		}
		ip := net.ParseIP(entry)
		if ip == nil {
			return ""
		}
		if !ip.IsLoopback() {
			return ip.String()
		}
	}
	return ""
}

// RateLimitKey is the key the per-IP rate limiters use: AttributionIP, or the
// TCP peer's address when that is empty (a request from this host), so a
// browser cannot pick its own bucket by sending X-Forwarded-For.
func RateLimitKey(r *http.Request) string {
	if ip := AttributionIP(r); ip != "" {
		return ip
	}
	if peer := peerIP(r); peer != nil {
		return peer.String()
	}
	return r.RemoteAddr
}

func peerIP(r *http.Request) net.IP {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return net.ParseIP(strings.TrimSpace(host))
}

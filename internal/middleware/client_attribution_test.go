package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ovander/backendkit/socrate"
	"github.com/stretchr/testify/assert"
)

func reqFrom(remote string, xff ...string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remote
	for _, v := range xff {
		r.Header.Add("X-Forwarded-For", v)
	}
	return r
}

func TestAttributionIP(t *testing.T) {
	cases := []struct {
		name   string
		remote string
		xff    []string
		want   string
	}{
		{"caddy appended the browser", "127.0.0.1:5000", []string{"203.0.113.7"}, "203.0.113.7"},
		{"browser-written entries on the left are ignored", "127.0.0.1:5000", []string{"1.2.3.4, 203.0.113.7"}, "203.0.113.7"},
		{"several headers read as one list", "127.0.0.1:5000", []string{"1.2.3.4", "203.0.113.7"}, "203.0.113.7"},
		{"loopback hops are skipped", "[::1]:5000", []string{"203.0.113.7, 127.0.0.1"}, "203.0.113.7"},
		{"unparsable rightmost entry gives nothing", "127.0.0.1:5000", []string{"1.2.3.4, garbage"}, ""},
		{"loopback without forwarded address gives nothing", "127.0.0.1:5000", nil, ""},
		{"non-loopback peer: headers ignored", "198.51.100.9:443", []string{"1.2.3.4"}, "198.51.100.9"},
		{"unparsable peer", "not-an-ip", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, AttributionIP(reqFrom(tc.remote, tc.xff...)))
		})
	}
}

func TestAttributionIPIgnoresXRealIP(t *testing.T) {
	r := reqFrom("127.0.0.1:5000", "203.0.113.7")
	r.Header.Set("X-Real-IP", "1.2.3.4")
	assert.Equal(t, "203.0.113.7", AttributionIP(r))
}

func TestRateLimitKey(t *testing.T) {
	assert.Equal(t, "203.0.113.7", RateLimitKey(reqFrom("127.0.0.1:5000", "1.2.3.4, 203.0.113.7")))
	assert.Equal(t, "198.51.100.9", RateLimitKey(reqFrom("198.51.100.9:443", "1.2.3.4")),
		"a direct client cannot choose its bucket with X-Forwarded-For")
	assert.Equal(t, "127.0.0.1", RateLimitKey(reqFrom("127.0.0.1:5000")))
}

func TestSocrateClientAttributionPutsResolvedIPInContext(t *testing.T) {
	var got socrate.ClientAttribution
	h := SocrateClientAttribution()(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got, _ = socrate.ClientAttributionFrom(r.Context())
	}))
	r := reqFrom("127.0.0.1:5000", "1.2.3.4, 203.0.113.7")
	r.Header.Set("User-Agent", "test-agent")
	h.ServeHTTP(httptest.NewRecorder(), r)
	assert.Equal(t, socrate.ClientAttribution{IP: "203.0.113.7", UserAgent: "test-agent"}, got)
}

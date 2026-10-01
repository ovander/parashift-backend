package router

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// No route hands an OAuth token to the browser (report row S3). The routes
// that did, POST /auth/callback, /auth/refresh and /auth/logout, are gone; the
// BFF (/bff) signs in and keeps the tokens on the server.

// TestRemovedTokenRoutesAreGone sends the former token routes, whatever the
// method, through the router with the BFF configured: each answers 404 or 405
// and never a body with a token field.
func TestRemovedTokenRoutesAreGone(t *testing.T) {
	e := newBFFEnv(t)
	for _, path := range []string{"/auth/callback", "/auth/refresh", "/auth/logout"} {
		for _, method := range []string{http.MethodPost, http.MethodGet} {
			body := strings.NewReader(`{"code":"c","codeVerifier":"v","refreshToken":"r"}`)
			r := e.browser(method, path, body)
			r.Header.Set("Content-Type", "application/json")
			w := e.do(r)

			assert.Contains(t, []int{http.StatusNotFound, http.StatusMethodNotAllowed}, w.Code,
				"%s %s answered %d: %s", method, path, w.Code, w.Body.String())
			assert.NotContains(t, strings.ToLower(w.Body.String()), "token\":", "%s %s", method, path)
		}
	}
	assert.Empty(t, e.soc.calls, "nothing reached Socrate")
}

// TestRoutesOutsideTheAPIArePinned lists every route outside /api/v1. A new
// one, an /auth route above all, must be added here on purpose: none of them
// may return a token, and the BFF routes return a session, not its tokens
// (TestBFF_SignInSetsHardenedCookieAndNoToken, TestBFF_SessionBodyNeverCarriesTokens).
func TestRoutesOutsideTheAPIArePinned(t *testing.T) {
	e := newBFFEnv(t)
	routes, ok := e.r.(chi.Routes)
	require.True(t, ok, "NewRouter returns a chi router")

	var got []string
	require.NoError(t, chi.Walk(routes, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if !strings.HasPrefix(route, "/api/v1/") {
			got = append(got, method+" "+route)
		}
		return nil
	}))
	sort.Strings(got)

	assert.Equal(t, []string{
		"GET /api/version",
		"GET /bff/callback",
		"GET /bff/login",
		"GET /bff/session",
		"GET /healthz",
		"GET /readyz",
		"POST /bff/logout",
	}, got)
}

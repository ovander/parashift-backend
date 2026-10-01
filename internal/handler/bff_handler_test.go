package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ovander/backendkit/bff"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The /bff flows run end to end in internal/router (router_bff_test.go); these
// cover the pieces that need the handler's internals.

func TestPendingLogins_SingleUseAndExpiry(t *testing.T) {
	now := time.Now()
	p := newPendingLogins(10)
	require.True(t, p.put("s1", pendingLogin{verifier: "v", expires: now.Add(time.Minute)}, now))

	pl, ok := p.take("s1", now)
	require.True(t, ok)
	assert.Equal(t, "v", pl.verifier)
	_, ok = p.take("s1", now)
	assert.False(t, ok, "single use")

	require.True(t, p.put("s2", pendingLogin{expires: now.Add(time.Minute)}, now))
	_, ok = p.take("s2", now.Add(time.Minute))
	assert.False(t, ok, "expired")
	_, ok = p.take("", now)
	assert.False(t, ok)
}

func TestPendingLogins_BoundedMemory(t *testing.T) {
	now := time.Now()
	p := newPendingLogins(2)
	require.True(t, p.put("a", pendingLogin{expires: now.Add(time.Minute)}, now))
	require.True(t, p.put("b", pendingLogin{expires: now.Add(time.Second)}, now))
	assert.False(t, p.put("c", pendingLogin{expires: now.Add(time.Minute)}, now), "full of live entries")
	assert.True(t, p.put("c", pendingLogin{expires: now.Add(time.Minute)}, now.Add(2*time.Second)), "expired ones make room")
}

func newTestBFFHandler(redirect string) *BFFHandler {
	gw := &bff.Gateway{Store: bff.NewMemoryStore(time.Minute, time.Hour), Cookie: bff.CookieConfig{Name: "s", Secure: true}}
	return NewBFFHandler(BFFOptions{Gateway: gw, Issuer: "https://idp.test/", ClientID: "c", RedirectURI: redirect, Logger: logrus.NewEntry(logrus.New())})
}

func TestBFFHandler_Sweep(t *testing.T) {
	h := newTestBFFHandler("https://app.test/bff/callback")
	assert.Equal(t, "https://idp.test/oauth/authorize", h.authorizeURL, "no double slash")
	h.pending.put("old", pendingLogin{expires: time.Now().Add(-time.Second)}, time.Now())
	h.pending.put("new", pendingLogin{expires: time.Now().Add(time.Minute)}, time.Now())
	h.Sweep()
	assert.Len(t, h.pending.m, 1)
}

func TestBFFHandler_DisabledWithoutRedirectURI(t *testing.T) {
	h := newTestBFFHandler("")
	for _, f := range []http.HandlerFunc{h.Login, h.Callback} {
		w := httptest.NewRecorder()
		f(w, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	}
}

func TestBFFHandler_LogoutWithoutSessionClearsCookie(t *testing.T) {
	h := newTestBFFHandler("https://app.test/bff/callback")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/bff/logout", nil)
	r.AddCookie(&http.Cookie{Name: "__Host-s", Value: "gone"})
	h.Logout(w, r)
	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.Contains(t, w.Header().Get("Set-Cookie"), "__Host-s=;")
}

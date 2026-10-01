package router

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ovander/backendkit/bff"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/socrate"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovander/parashift/internal/config"
	"github.com/ovander/parashift/internal/handler"
	"github.com/ovander/parashift/internal/middleware"
	"github.com/ovander/parashift/internal/service"
)

// End-to-end tests of the Backend-for-Frontend through the production router,
// against a fake Socrate that records every call: sign-in, callback
// rejections, session, CSRF, logout, refresh, the bearer and e-mail the API
// sees, cookie attributes and the address Socrate is told.

const (
	bffCookie     = "__Host-parashift_session"
	bindingCookie = "__Host-parashift_session_login"
	bffRedirect   = "https://app.test/bff/callback"
	goodCode      = "good-code"
)

// socrateCall is one request the fake Socrate received.
type socrateCall struct {
	Path      string
	Form      url.Values
	XFF       string
	XRealIP   string
	UserAgent string
}

// fakeSocrate plays Socrate's OAuth endpoints and /api/profile: it accepts one
// code bound to the PKCE challenge of the last /bff/login, rotates refresh
// tokens, and records every call.
type fakeSocrate struct {
	*httptest.Server
	mu        sync.Mutex
	challenge string
	refresh   string // the refresh token it accepts
	verified  bool   // what /api/profile says about the e-mail
	n         int
	calls     []socrateCall
}

func newFakeSocrate(t *testing.T) *fakeSocrate {
	t.Helper()
	f := &fakeSocrate{verified: true}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeSocrate) tokens() string {
	f.n++
	f.refresh = fmt.Sprintf("rt-%d", f.n)
	return fmt.Sprintf(`{"access_token":"at-%d","refresh_token":"%s","token_type":"Bearer","expires_in":900}`, f.n, f.refresh)
}

func (f *fakeSocrate) serve(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, socrateCall{
		Path: r.URL.Path, Form: r.PostForm, XFF: r.Header.Get("X-Forwarded-For"),
		XRealIP: r.Header.Get("X-Real-IP"), UserAgent: r.Header.Get("User-Agent"),
	})
	w.Header().Set("Content-Type", "application/json")
	invalid := func() {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"invalid_grant"}`)
	}
	switch r.URL.Path {
	case "/oauth/token":
		switch r.PostForm.Get("grant_type") {
		case "authorization_code":
			if r.PostForm.Get("code") != goodCode || r.PostForm.Get("redirect_uri") != bffRedirect ||
				bff.S256Challenge(r.PostForm.Get("code_verifier")) != f.challenge || r.PostForm.Get("client_secret") != "secret" {
				invalid()
				return
			}
			_, _ = io.WriteString(w, f.tokens())
		case "refresh_token":
			if r.PostForm.Get("refresh_token") != f.refresh {
				invalid()
				return
			}
			_, _ = io.WriteString(w, f.tokens())
		default:
			invalid()
		}
	case "/api/profile":
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer at-") {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = fmt.Fprintf(w, `{"id":42,"email":"ada@example.test","name":"Ada Lovelace","is_verified":%t}`, f.verified)
	case "/oauth/revoke":
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

func (f *fakeSocrate) callsTo(path string) []socrateCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []socrateCall
	for _, c := range f.calls {
		if c.Path == path {
			out = append(out, c)
		}
	}
	return out
}

// apiSeen is what the auth middleware (a stub standing in for jwtauth) saw on
// the last /api/v1 request that reached it.
type apiSeen struct {
	mu            sync.Mutex
	authorization string
	email         string
	calls         int
}

type bffEnv struct {
	t     *testing.T
	r     http.Handler
	soc   *fakeSocrate
	store *bff.MemoryStore
	seen  *apiSeen
	now   atomic.Int64 // unix nanos; the handler's clock
	ip    atomic.Int32 // a fresh browser address per request, so the rate limiter never trips
}

func newBFFEnv(t *testing.T) *bffEnv {
	t.Helper()
	e := &bffEnv{t: t, soc: newFakeSocrate(t), seen: &apiSeen{}}
	e.now.Store(time.Now().UnixNano())
	sc, err := socrate.NewClient(socrate.ClientConfig{BaseURL: e.soc.URL, AdminBaseURL: e.soc.URL, ClientID: "client", ClientSecret: "secret"})
	require.NoError(t, err)
	lg := logrus.New()
	lg.SetOutput(io.Discard)
	le := logrus.NewEntry(lg)

	e.store = bff.NewMemoryStore(30*time.Minute, 8*time.Hour)
	gw := &bff.Gateway{
		Store:     e.store,
		Cookie:    bff.CookieConfig{Name: "parashift_session", Secure: true, MaxAge: int((8 * time.Hour).Seconds())},
		Refresher: middleware.EndWithoutRefreshToken(sc),
	}
	h := handler.NewBFFHandler(handler.BFFOptions{
		Gateway: gw, Auth: service.NewSessionAuthService(sc, sc, le),
		Issuer: e.soc.URL, ClientID: "client", RedirectURI: bffRedirect, Logger: le,
		Now: func() time.Time { return time.Unix(0, e.now.Load()) },
	})

	// The auth stub ends every /api/v1 request: it records the bearer and the
	// e-mail the session middleware handed on, and answers 204.
	authStub := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			e.seen.mu.Lock()
			e.seen.authorization = r.Header.Get("Authorization")
			e.seen.email = ctxutil.GetUserEmail(r.Context())
			e.seen.calls++
			e.seen.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		})
	}
	e.r = NewRouter(&config.Config{MaxRequestBodyBytes: 1 << 20}, &handler.HandlerBundle{}, Middleware{
		Auth:   authStub,
		RBAC:   middleware.NewRBACMiddleware(le),
		Logger: lg,
		BFF:    &BFF{Handler: h, Session: middleware.NewSessionAuth(gw, true, le)},
	})
	return e
}

// browser returns a request as it arrives from Caddy on loopback, for a
// browser at a fresh address.
func (e *bffEnv) browser(method, target string, body io.Reader) *http.Request {
	r := httptest.NewRequest(method, target, body)
	r.RemoteAddr = "127.0.0.1:40000"
	r.Header.Set("X-Forwarded-For", fmt.Sprintf("198.51.100.%d", e.ip.Add(1)%250+1))
	r.Header.Set("User-Agent", "Mozilla/5.0 (bff test)")
	return r
}

func (e *bffEnv) do(r *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, r)
	return w
}

// cookie returns the named cookie a response sets, or nil.
func cookie(w *httptest.ResponseRecorder, name string) *http.Cookie {
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// login runs GET /bff/login and returns the state and the binding cookie. The
// fake Socrate is told the PKCE challenge, as /oauth/authorize would be.
func (e *bffEnv) login(returnTo string) (state string, binding *http.Cookie) {
	e.t.Helper()
	w := e.do(e.browser(http.MethodGet, "/bff/login?return_to="+url.QueryEscape(returnTo), nil))
	require.Equal(e.t, http.StatusFound, w.Code)
	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(e.t, err)
	e.soc.mu.Lock()
	e.soc.challenge = loc.Query().Get("code_challenge")
	e.soc.mu.Unlock()
	binding = cookie(w, bindingCookie)
	require.NotNil(e.t, binding)
	return loc.Query().Get("state"), binding
}

func (e *bffEnv) callback(state string, binding *http.Cookie, code string) *httptest.ResponseRecorder {
	r := e.browser(http.MethodGet, "/bff/callback?code="+url.QueryEscape(code)+"&state="+url.QueryEscape(state), nil)
	if binding != nil {
		r.AddCookie(binding)
	}
	return e.do(r)
}

// signIn runs the whole flow and returns the session cookie.
func (e *bffEnv) signIn() *http.Cookie {
	e.t.Helper()
	state, binding := e.login("/stores/1/planner")
	w := e.callback(state, binding, goodCode)
	require.Equal(e.t, http.StatusFound, w.Code, w.Body.String())
	c := cookie(w, bffCookie)
	require.NotNil(e.t, c)
	return c
}

func (e *bffEnv) session(c *http.Cookie) handler.SessionResponse {
	e.t.Helper()
	r := e.browser(http.MethodGet, "/bff/session", nil)
	if c != nil {
		r.AddCookie(c)
	}
	w := e.do(r)
	require.Equal(e.t, http.StatusOK, w.Code)
	var s handler.SessionResponse
	require.NoError(e.t, json.Unmarshal(w.Body.Bytes(), &s))
	return s
}

// api sends a GET to /api/v1/me with the session cookie and returns the status.
func (e *bffEnv) api(c *http.Cookie, bearer string) int {
	r := e.browser(http.MethodGet, "/api/v1/me", nil)
	if c != nil {
		r.AddCookie(c)
	}
	if bearer != "" {
		r.Header.Set("Authorization", "Bearer "+bearer)
	}
	return e.do(r).Code
}

func TestBFF_LoginRedirectsToAuthorizeWithPKCE(t *testing.T) {
	e := newBFFEnv(t)
	w := e.do(e.browser(http.MethodGet, "/bff/login?return_to=/stores/1/planner", nil))

	require.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	loc, err := url.Parse(w.Header().Get("Location"))
	require.NoError(t, err)
	assert.Equal(t, e.soc.URL+"/oauth/authorize", loc.Scheme+"://"+loc.Host+loc.Path)
	q := loc.Query()
	assert.Equal(t, "code", q.Get("response_type"))
	assert.Equal(t, "client", q.Get("client_id"))
	assert.Equal(t, bffRedirect, q.Get("redirect_uri"))
	assert.Equal(t, "openid email profile api", q.Get("scope"))
	assert.Equal(t, "S256", q.Get("code_challenge_method"))
	assert.Len(t, q.Get("code_challenge"), 43)
	assert.GreaterOrEqual(t, len(q.Get("state")), 43, "32 random bytes")

	b := cookie(w, bindingCookie)
	require.NotNil(t, b)
	assert.True(t, b.HttpOnly && b.Secure)
	assert.Equal(t, http.SameSiteLaxMode, b.SameSite, "Lax: the callback is a navigation from Socrate")
	assert.Nil(t, cookie(w, bffCookie), "no session before the callback")
	assert.Empty(t, e.soc.callsTo("/oauth/token"))
}

func TestBFF_SignInSetsHardenedCookieAndNoToken(t *testing.T) {
	e := newBFFEnv(t)
	state, binding := e.login("/stores/7/planner?week=2026-W40")
	w := e.callback(state, binding, goodCode)

	require.Equal(t, http.StatusFound, w.Code, w.Body.String())
	assert.Equal(t, "/stores/7/planner?week=2026-W40", w.Header().Get("Location"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))

	var raw string
	for _, h := range w.Header().Values("Set-Cookie") {
		if strings.HasPrefix(h, bffCookie+"=") {
			raw = h
		}
	}
	require.NotEmpty(t, raw, "session cookie set")
	for _, attr := range []string{"Path=/", "HttpOnly", "Secure", "SameSite=Strict"} {
		assert.Contains(t, raw, attr)
	}
	assert.NotContains(t, strings.ToLower(raw), "domain=", "__Host- forbids Domain")

	// The code was exchanged with the verifier matching the challenge, for the BFF redirect URI.
	ex := e.soc.callsTo("/oauth/token")
	require.Len(t, ex, 1)
	assert.Equal(t, "authorization_code", ex[0].Form.Get("grant_type"))
	assert.Equal(t, bffRedirect, ex[0].Form.Get("redirect_uri"))

	// No token in anything the browser receives.
	for _, h := range w.Header() {
		for _, v := range h {
			assert.NotContains(t, v, "at-1")
			assert.NotContains(t, v, "rt-1")
		}
	}
	assert.NotContains(t, w.Body.String(), "at-1")

	s := e.session(cookie(w, bffCookie))
	assert.True(t, s.Authenticated)
	require.NotNil(t, s.User)
	assert.Equal(t, "42", s.User.Sub)
	assert.Equal(t, "ada@example.test", s.User.Email)
	assert.Equal(t, "Ada Lovelace", s.User.Name)
	assert.NotEmpty(t, s.CSRF)
}

func TestBFF_SessionBodyNeverCarriesTokens(t *testing.T) {
	e := newBFFEnv(t)
	c := e.signIn()
	r := e.browser(http.MethodGet, "/bff/session", nil)
	r.AddCookie(c)
	w := e.do(r)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	body := w.Body.String()
	for _, secret := range []string{"at-1", "rt-1", "access", "refresh", "token\""} {
		assert.NotContains(t, body, secret)
	}
	assert.False(t, e.session(nil).Authenticated)
}

func TestBFF_CallbackRejections(t *testing.T) {
	type attempt func(e *bffEnv) *httptest.ResponseRecorder
	cases := map[string]attempt{
		"unknown state": func(e *bffEnv) *httptest.ResponseRecorder {
			_, binding := e.login("/")
			return e.callback("not-a-state-we-issued", binding, goodCode)
		},
		"no state": func(e *bffEnv) *httptest.ResponseRecorder {
			_, binding := e.login("/")
			return e.callback("", binding, goodCode)
		},
		"expired state": func(e *bffEnv) *httptest.ResponseRecorder {
			state, binding := e.login("/")
			e.now.Add(int64(bff.DefaultLoginBindingTTL + time.Second))
			return e.callback(state, binding, goodCode)
		},
		"missing binding cookie": func(e *bffEnv) *httptest.ResponseRecorder {
			state, _ := e.login("/")
			return e.callback(state, nil, goodCode)
		},
		"foreign binding cookie": func(e *bffEnv) *httptest.ResponseRecorder {
			state, _ := e.login("/")         // the attacker's flow
			_, victimBinding := e.login("/") // the victim's browser
			return e.callback(state, victimBinding, goodCode)
		},
		"Socrate refused": func(e *bffEnv) *httptest.ResponseRecorder {
			state, binding := e.login("/")
			r := e.browser(http.MethodGet, "/bff/callback?error=access_denied&state="+url.QueryEscape(state), nil)
			r.AddCookie(binding)
			return e.do(r)
		},
		"bad code": func(e *bffEnv) *httptest.ResponseRecorder {
			state, binding := e.login("/")
			return e.callback(state, binding, "stolen-code")
		},
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) {
			e := newBFFEnv(t)
			w := run(e)
			require.Equal(t, http.StatusFound, w.Code)
			assert.True(t, strings.HasPrefix(w.Header().Get("Location"), "/login?error="), w.Header().Get("Location"))
			assert.Nil(t, cookie(w, bffCookie), "no session cookie")
			if c := cookie(w, bindingCookie); c != nil {
				assert.Negative(t, c.MaxAge, "the binding cookie is cleared")
			}
			if name != "bad code" {
				assert.Empty(t, e.soc.callsTo("/oauth/token"), "nothing reaches Socrate")
			}
		})
	}
}

func TestBFF_StateIsSingleUse(t *testing.T) {
	e := newBFFEnv(t)
	state, binding := e.login("/")
	first := e.callback(state, binding, goodCode)
	require.NotNil(t, cookie(first, bffCookie))

	replay := e.callback(state, binding, goodCode)
	assert.True(t, strings.HasPrefix(replay.Header().Get("Location"), "/login?error="))
	assert.Nil(t, cookie(replay, bffCookie))
	assert.Len(t, e.soc.callsTo("/oauth/token"), 1, "the replay never reaches Socrate")
}

func TestBFF_ReturnToIsSanitised(t *testing.T) {
	for in, want := range map[string]string{
		"//evil.example":                "/",
		"https://evil.example":          "/",
		"/\\evil.example":               "/",
		"javascript:alert(1)":           "/",
		"/bff/logout":                   "/",
		"/stores/1/planner?week=2026#x": "/stores/1/planner?week=2026#x",
		"":                              "/",
	} {
		e := newBFFEnv(t)
		state, binding := e.login(in)
		w := e.callback(state, binding, goodCode)
		require.NotNil(t, cookie(w, bffCookie), in)
		assert.Equal(t, want, w.Header().Get("Location"), "return_to %q", in)
	}
}

func TestBFF_ProtectedRoutesNeedASessionOrABearer(t *testing.T) {
	e := newBFFEnv(t)
	for _, target := range []string{"/api/v1/me", "/api/v1/stores/me", "/api/v1/admin/stats"} {
		w := e.do(e.browser(http.MethodGet, target, nil))
		assert.Equal(t, http.StatusUnauthorized, w.Code, target)
		assert.Contains(t, w.Body.String(), "authentication required", target)
	}
	assert.Zero(t, e.seen.calls, "nothing reached the auth middleware")

	// A stale session cookie without a bearer is cleared.
	r := e.browser(http.MethodGet, "/api/v1/me", nil)
	r.AddCookie(&http.Cookie{Name: bffCookie, Value: "gone"})
	w := e.do(r)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	if c := cookie(w, bffCookie); assert.NotNil(t, c) {
		assert.Negative(t, c.MaxAge)
	}
}

// During the transition the current SPA still sends its own bearer, without a
// session: it goes to the auth middleware untouched.
func TestBFF_BearerWithoutSessionStillPasses(t *testing.T) {
	e := newBFFEnv(t)
	assert.Equal(t, http.StatusNoContent, e.api(nil, "spa-token"))
	assert.Equal(t, "Bearer spa-token", e.seen.authorization)
	assert.Empty(t, e.seen.email)
}

// With a session, the session's access token replaces any bearer the browser
// sends, and the verified e-mail reaches the API (the invite claim checks it).
func TestBFF_SessionBecomesTheBearer(t *testing.T) {
	e := newBFFEnv(t)
	c := e.signIn()
	assert.Equal(t, http.StatusNoContent, e.api(c, "forged"))
	assert.Equal(t, "Bearer at-1", e.seen.authorization)
	assert.Equal(t, "ada@example.test", e.seen.email)
}

func TestBFF_UnverifiedEmailNeverReachesTheAPI(t *testing.T) {
	e := newBFFEnv(t)
	e.soc.mu.Lock()
	e.soc.verified = false
	e.soc.mu.Unlock()
	c := e.signIn()

	s := e.session(c)
	require.NotNil(t, s.User)
	assert.Empty(t, s.User.Email)
	assert.Equal(t, http.StatusNoContent, e.api(c, ""))
	assert.Empty(t, e.seen.email)
}

func TestBFF_CSRFOnTheAPI(t *testing.T) {
	e := newBFFEnv(t)
	c := e.signIn()
	csrf := e.session(c).CSRF

	for _, token := range []string{"", "wrong", csrf + "x"} {
		r := e.browser(http.MethodPost, "/api/v1/me/sessions/revoke", strings.NewReader(`{}`))
		r.AddCookie(c)
		if token != "" {
			r.Header.Set("X-CSRF-Token", token)
		}
		assert.Equal(t, http.StatusForbidden, e.do(r).Code, "token %q", token)
	}
	assert.Zero(t, e.seen.calls)

	r := e.browser(http.MethodPost, "/api/v1/me/sessions/revoke", strings.NewReader(`{}`))
	r.AddCookie(c)
	r.Header.Set("X-CSRF-Token", csrf)
	assert.Equal(t, http.StatusNoContent, e.do(r).Code)
}

func TestBFF_LogoutRevokesAndEndsSession(t *testing.T) {
	e := newBFFEnv(t)
	c := e.signIn()
	csrf := e.session(c).CSRF

	// Without the CSRF token nothing happens.
	r := e.browser(http.MethodPost, "/bff/logout", nil)
	r.AddCookie(c)
	require.Equal(t, http.StatusForbidden, e.do(r).Code)
	assert.Empty(t, e.soc.callsTo("/oauth/revoke"))
	assert.True(t, e.session(c).Authenticated)

	r = e.browser(http.MethodPost, "/bff/logout", nil)
	r.AddCookie(c)
	r.Header.Set("X-CSRF-Token", csrf)
	w := e.do(r)
	require.Equal(t, http.StatusNoContent, w.Code)
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))

	revoked := e.soc.callsTo("/oauth/revoke")
	require.Len(t, revoked, 1)
	assert.Equal(t, "rt-1", revoked[0].Form.Get("token"), "the refresh token is revoked")
	cleared := cookie(w, bffCookie)
	require.NotNil(t, cleared)
	assert.Negative(t, cleared.MaxAge)

	// The old cookie is dead everywhere.
	assert.False(t, e.session(c).Authenticated)
	assert.Equal(t, http.StatusUnauthorized, e.api(c, ""))
}

// TestBFF_SocrateSeesTheResolvedBrowserAddress: X-Forwarded-For is trusted from
// loopback (Caddy) only, and X-Real-IP never reaches Socrate.
func TestBFF_SocrateSeesTheResolvedBrowserAddress(t *testing.T) {
	cases := []struct {
		name, remote, xff, want string
	}{
		{"direct peer claiming 6.6.6.6", "203.0.113.9:5555", "6.6.6.6", "203.0.113.9"},
		{"through Caddy on loopback", "127.0.0.1:40000", "198.51.100.23", "198.51.100.23"},
		{"browser-supplied entry before Caddy's", "127.0.0.1:40000", "6.6.6.6, 198.51.100.24", "198.51.100.24"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := newBFFEnv(t)
			state, binding := e.login("/")
			r := httptest.NewRequest(http.MethodGet, "/bff/callback?code="+goodCode+"&state="+url.QueryEscape(state), nil)
			r.AddCookie(binding)
			r.RemoteAddr = tc.remote
			r.Header.Set("X-Forwarded-For", tc.xff)
			r.Header.Set("X-Real-IP", "7.7.7.7")
			r.Header.Set("User-Agent", "Mozilla/5.0 (attribution)")
			w := e.do(r)
			c := cookie(w, bffCookie)
			require.NotNil(t, c, w.Header().Get("Location"))

			ex := e.soc.callsTo("/oauth/token")
			require.Len(t, ex, 1)
			assert.Equal(t, tc.want, ex[0].XFF)
			assert.Empty(t, ex[0].XRealIP, "X-Real-IP is never forwarded")
			assert.Equal(t, "Mozilla/5.0 (attribution)", ex[0].UserAgent)

			// The revocation at logout is attributed the same way.
			csrf := e.session(c).CSRF
			r = httptest.NewRequest(http.MethodPost, "/bff/logout", nil)
			r.AddCookie(c)
			r.Header.Set("X-CSRF-Token", csrf)
			r.RemoteAddr = tc.remote
			r.Header.Set("X-Forwarded-For", tc.xff)
			r.Header.Set("X-Real-IP", "7.7.7.7")
			require.Equal(t, http.StatusNoContent, e.do(r).Code)
			rv := e.soc.callsTo("/oauth/revoke")
			require.Len(t, rv, 1)
			assert.Equal(t, tc.want, rv[0].XFF)
			assert.Empty(t, rv[0].XRealIP)
		})
	}
}

// TestBFF_RefreshOnTheAPIIsAttributedAndRotated: an access token about to
// expire is refreshed by the session middleware on an API call; Socrate is
// told the browser's address and the rotated refresh token is kept.
func TestBFF_RefreshOnTheAPIIsAttributedAndRotated(t *testing.T) {
	e := newBFFEnv(t)
	c := e.signIn()
	s, ok := e.store.Get(c.Value)
	require.True(t, ok)
	s.SetTokens(&socrate.TokenSet{AccessToken: "at-1", RefreshToken: "rt-1", ExpiresIn: 5}, time.Now()) // inside the refresh leeway

	r := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	r.AddCookie(c)
	r.RemoteAddr = "127.0.0.1:40000"
	r.Header.Set("X-Forwarded-For", "198.51.100.77")
	r.Header.Set("X-Real-IP", "7.7.7.7")
	require.Equal(t, http.StatusNoContent, e.do(r).Code)
	assert.Equal(t, "Bearer at-2", e.seen.authorization, "the fresh access token")

	var refreshes []socrateCall
	for _, call := range e.soc.callsTo("/oauth/token") {
		if call.Form.Get("grant_type") == "refresh_token" {
			refreshes = append(refreshes, call)
		}
	}
	require.Len(t, refreshes, 1)
	assert.Equal(t, "rt-1", refreshes[0].Form.Get("refresh_token"))
	assert.Equal(t, "198.51.100.77", refreshes[0].XFF)
	assert.Empty(t, refreshes[0].XRealIP)
	assert.Equal(t, "rt-2", s.RefreshToken(), "rotated token kept")

	// A refresh Socrate rejects ends the session.
	e.soc.mu.Lock()
	e.soc.refresh = "revoked-elsewhere"
	e.soc.mu.Unlock()
	s.SetTokens(&socrate.TokenSet{AccessToken: "at-2", RefreshToken: "rt-2", ExpiresIn: 5}, time.Now())
	assert.Equal(t, http.StatusUnauthorized, e.api(c, ""))
	assert.False(t, e.session(c).Authenticated)
}
